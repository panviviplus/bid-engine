package utils

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"

	"bid-engine/pkg/repo/converter"
)

// SourceFile 源文件结构（本地文件），用于表示从对象存储下载到本地的文件或本地已有文件。
// 注意：为避免内存占用，文件内容不在此结构中，仅保存必要的元信息。
type SourceFile struct {
	// 本地绝对或相对路径
	Path string
	// 文件名（不含路径）
	Name string
	// 扩展名（含点，例如 .pdf/.doc/.docx）
	Ext string
	// 文件大小（字节），可能在构建时未知；函数内部会补齐。
	Size int64
}

// PDFChunk 是一个物理 PDF 页块。PageStart/PageEnd 使用原文 1-based 页码。
type PDFChunk struct {
	SourceFile
	PageStart int
	PageEnd   int
}

// PageCount 返回 PDF 的真实页数，不读取页面内容到内存。
func PageCount(pdfPath string) (int, error) {
	count, err := api.PageCountFile(pdfPath)
	if err != nil {
		return 0, fmt.Errorf("读取 PDF 页数失败: %w", err)
	}
	if count <= 0 {
		return 0, fmt.Errorf("PDF 页数无效: %d", count)
	}
	return count, nil
}

// SplitPDFByRanges 将 PDF 切成连续的实体页块。默认每块 10 页；若块文件超过
// maxChunkBytes，会递归二分。为避免超大图片文档被拆成大量单页块（页块数爆炸、
// Docling 请求数剧增），二分下限为 minSplitPages（5 页）：10 页块超阈值拆成
// 两个 5 页块，5 页及以下不再继续拆分。
func SplitPDFByRanges(pdfPath string, pagesPerChunk int, maxChunkBytes int64) ([]*PDFChunk, error) {
	return SplitPDFByRangesContext(context.Background(), pdfPath, pagesPerChunk, maxChunkBytes)
}

// minSplitPages 是字节超阈值时二分的最小页数下限：低于该页数的块保持原状，
// 避免“10 页 → 5 页”之外再无限切小。
const minSplitPages = 5

// SplitPDFByRangesContext 与 SplitPDFByRanges 相同，但会在每个物理页块之间响应取消。
// pdfcpu 的单次 TrimFile 不支持 context，因此取消会在当前页块落盘后、下一页块开始前生效。
func SplitPDFByRangesContext(ctx context.Context, pdfPath string, pagesPerChunk int, maxChunkBytes int64) ([]*PDFChunk, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if pagesPerChunk <= 0 {
		pagesPerChunk = 10
	}
	if maxChunkBytes <= 0 {
		maxChunkBytes = 20 << 20
	}
	pageCount, err := PageCount(pdfPath)
	if err != nil {
		return nil, err
	}

	baseDir := filepath.Dir(pdfPath)
	baseName := strings.TrimSuffix(filepath.Base(pdfPath), filepath.Ext(pdfPath))
	outDir, err := os.MkdirTemp(baseDir, "chunks-"+baseName+"-")
	if err != nil {
		return nil, fmt.Errorf("创建页块目录失败: %w", err)
	}

	conf := model.NewDefaultConfiguration()
	var chunks []*PDFChunk
	var emit func(int, int) error
	emit = func(start, end int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		outPath := filepath.Join(outDir, fmt.Sprintf("%s_pages_%04d_%04d.pdf", baseName, start, end))
		selection := []string{fmt.Sprintf("%d-%d", start, end)}
		if start == end {
			selection = []string{fmt.Sprintf("%d", start)}
		}
		if err := api.TrimFile(pdfPath, outPath, selection, conf); err != nil {
			return fmt.Errorf("切分 PDF 页 %d-%d 失败: %w", start, end, err)
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		info, err := os.Stat(outPath)
		if err != nil {
			return fmt.Errorf("读取页块 %d-%d 失败: %w", start, end, err)
		}
		if info.Size() > maxChunkBytes && end-start+1 >= 2*minSplitPages {
			if err := os.Remove(outPath); err != nil {
				return fmt.Errorf("清理超大页块失败: %w", err)
			}
			middle := start + (end-start)/2
			if err := emit(start, middle); err != nil {
				return err
			}
			return emit(middle+1, end)
		}
		chunks = append(chunks, &PDFChunk{
			SourceFile: SourceFile{Path: outPath, Name: filepath.Base(outPath), Ext: ".pdf", Size: info.Size()},
			PageStart:  start,
			PageEnd:    end,
		})
		return nil
	}

	for start := 1; start <= pageCount; start += pagesPerChunk {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		end := start + pagesPerChunk - 1
		if end > pageCount {
			end = pageCount
		}
		if err := emit(start, end); err != nil {
			return nil, err
		}
	}
	return chunks, nil
}

// SplitPDFByPage 按页切分一个本地文件（PDF/Office 文档）。
// 入参：pdfPath 为本地文件路径；
// 能力：
// - 若输入为 .docx 或 .doc，先转换为 .pdf，再按页切分；
// - 对大文件友好（不加载到内存，采用 pdfcpu 按页提取）。
// 返回：每页生成的单页 PDF 源文件列表。
func SplitPDFByPage(pdfPath string) ([]*SourceFile, error) {
	if strings.TrimSpace(pdfPath) == "" {
		return nil, fmt.Errorf("输入文件路径为空")
	}
	if _, err := os.Stat(pdfPath); err != nil {
		return nil, fmt.Errorf("输入文件不存在或不可访问: %w", err)
	}
	ext := strings.ToLower(filepath.Ext(pdfPath))

	// 如果是 Office 文档，先转换成 PDF
	workPDF := pdfPath
	if ext == ".docx" || ext == ".doc" {
		pdfPath2, err := ConvertOfficeToPDF(pdfPath)
		if err != nil {
			return nil, fmt.Errorf("Office 转 PDF 失败: %w", err)
		}
		workPDF = pdfPath2
		ext = ".pdf"
	}

	// 读取 PDF 页数
	ctx, err := api.ReadContextFile(workPDF)
	if err != nil {
		return nil, fmt.Errorf("读取 PDF 失败: %w", err)
	}
	pageCount := ctx.PageCount
	if pageCount <= 0 {
		return nil, fmt.Errorf("PDF 页数无效: %d", pageCount)
	}

	// 输出目录：<输入文件所在目录>/pages-<基名>-<时间戳>
	baseDir := filepath.Dir(workPDF)
	baseName := strings.TrimSuffix(filepath.Base(workPDF), filepath.Ext(workPDF))
	outDir := filepath.Join(baseDir, fmt.Sprintf("pages-%s-%s", baseName, time.Now().Format("20060102150405")))
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建输出目录失败: %w", err)
	}

	// 按页切分（1-N）
	conf := model.NewDefaultConfiguration()
	pageSel := fmt.Sprintf("1-%d", pageCount)
	if err := api.ExtractPagesFile(workPDF, outDir, []string{pageSel}, conf); err != nil {
		return nil, fmt.Errorf("按页切分失败: %w", err)
	}

	// 组装返回的源文件列表：命名 <基名>_page_<N>.pdf
	files := make([]*SourceFile, 0, pageCount)
	for i := 1; i <= pageCount; i++ {
		p := filepath.Join(outDir, fmt.Sprintf("%s_page_%d.pdf", baseName, i))
		fi2, err := os.Stat(p)
		if err != nil {
			return nil, fmt.Errorf("生成文件缺失: %s, err=%v", p, err)
		}
		files = append(files, &SourceFile{
			Path: p,
			Name: filepath.Base(p),
			Ext:  ".pdf",
			Size: fi2.Size(),
		})
	}
	return files, nil
}

// ConvertOfficeToPDF 将 .doc/.docx 转换为 .pdf。
// 转换由独立的 doc-converter 依赖服务完成，返回的 PDF 为本地临时文件，调用方负责删除。
func ConvertOfficeToPDF(docPath string) (string, error) {
	return ConvertOfficeToPDFContext(context.Background(), docPath)
}

// ConvertOfficeToPDFContext 调用文档转换服务完成 Office → PDF。
func ConvertOfficeToPDFContext(ctx context.Context, docPath string) (string, error) {
	if _, err := os.Stat(docPath); err != nil {
		return "", fmt.Errorf("输入 Office 文件不存在: %w", err)
	}
	return converter.GetInstance().ConvertToPDF(ctx, docPath)
}
