package pdf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"bid-engine/pkg/entity"
	"bid-engine/pkg/repo/converter"
)

// SimpleData 简化的PDF数据
type SimpleData struct {
	Id        string `json:"id"`        // PDF解析任务的唯一标识ID
	Author    string `json:"author"`    // PDF文档的作者信息
	CharCount int64  `json:"charCount"` // 整个文档的字符总数
	PageCount int64  `json:"pageCount"` // 文档总页数
	Html      string `json:"html"`      // 文档的HTML富文本表示
}

// Data 整个PDF数据
type Data struct {
	Id        string  `json:"id"`        // PDF解析任务的唯一标识ID
	Author    string  `json:"author"`    // PDF文档的作者信息
	CharCount int64   `json:"charCount"` // 整个文档的字符总数
	PageCount int64   `json:"pageCount"` // 文档总页数
	Pages     []*Page `json:"pages"`     // 每页的详细解析数据
	Html      string  `json:"html"`      // 文档的HTML富文本表示
}

// Page 每页数据
type Page struct {
	BlockCount int64    `json:"blockCount"` // 当前页面的文本块数量
	Blocks     []*Block `json:"blocks"`     // 页面内的所有文本块
	CharCount  int64    `json:"charCount"`  // 当前页面的字符数
	Height     int32    `json:"height"`     // 页面高度（像素）
	PageIndex  int64    `json:"pageIndex"`  // 页面索引（从0开始）
	Width      int32    `json:"width"`      // 页面宽度（像素）
}

// Block 每块数据
type Block struct {
	BlockIndex     int64     `json:"blockIndex"`     // 文本块在页面中的索引
	CharCount      int64     `json:"charCount"`      // 当前文本块的字符数
	CharPositions  [][]int64 `json:"charPositions"`  // 字符在页面中的坐标位置
	Chars          string    `json:"chars"`          // 文本块的实际文字内容
	TableIDs       []int32   `json:"tableIDs"`       // 表格ID列表（如果包含表格）
	TablePositions [][]int32 `json:"tablePositions"` // 表格在页面中的位置坐标
}

// RespBody 返回结构体
type RespBody struct {
	Code      int64  `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
	Data      *Data  `json:"data"`
}

var (
	_pdfBaseURL, _pdfAppID, _pdfAppSecret string
)

const (
	//parseAPI   = "/pdf/with-table-parse"
	parseAPI   = "/pdf/with-structure-table"
	getHtmlAPI = "/pdf/async/all?id=%s&format=HTML"
)

func (s *svcImpl) GetHtml(c *gin.Context, id string) (string, error) {
	log := s.logger.With(entity.Ctx(c)...)
	url := _pdfBaseURL + fmt.Sprintf(getHtmlAPI, id)
	method := "GET"

	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)

	if err != nil {
		log.Warnw("call NewRequest", "err", err)
		return "", err
	}
	req.Header.Set("X-App-Id", _pdfAppID)
	req.Header.Set("X-App-Secret", _pdfAppSecret)

	res, err := client.Do(req)
	if err != nil {
		log.Warnw("call Do", "err", err)
		return "", err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Warnw("call ReadAll", "err", err)
		return "", err
	}
	return string(body), nil
}

// Parse  获取pdf解析待处理任务
func (s *svcImpl) Parse(c *gin.Context, fileName string) (*Data, error) {
	log := s.logger.With(entity.Ctx(c)...)
	method := "POST"
	payload := &bytes.Buffer{}
	writer := multipart.NewWriter(payload)
	file, err := os.Open(fileName)
	if err != nil {
		log.Warnw("call Open", "err", err)
		return nil, err
	}
	defer file.Close()
	part, err := writer.CreateFormFile("file", filepath.Base(fileName))
	if err != nil {
		log.Warnw("call CreateFormFile", "err", err)
		return nil, err
	}
	_, err = io.Copy(part, file)
	if err != nil {
		log.Warnw("call Copy", "err", err)
		return nil, err
	}
	err = writer.WriteField("parse_table_content", "true")
	if err != nil {
		log.Warnw("call WriteField", "err", err)
		return nil, err
	}
	err = writer.Close()
	if err != nil {
		log.Warnw("call Close", "err", err)
		return nil, err
	}

	req, err := http.NewRequest(method, _pdfBaseURL+parseAPI, payload)
	if err != nil {
		log.Warnw("call NewRequest", "err", err)
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-App-Id", _pdfAppID)
	req.Header.Set("X-App-Secret", _pdfAppSecret)
	res, err := s.client.Do(req)
	if err != nil {
		log.Warnw("call Do", "err", err)
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Warnw("call ReadAll", "err", err)
		return nil, err
	}
	log.Infow("response", "body", string(body))
	if res.StatusCode != http.StatusOK {
		err = fmt.Errorf("http code: %d; msg:%s", res.StatusCode, string(body))
		log.Warnw("http code not 200", "status", res.StatusCode, "body", string(body), "err", err)
		return nil, err
	}
	var result *RespBody
	err = json.Unmarshal(body, &result)
	if err != nil {
		log.Warnw("call Unmarshal", "err", err)
		return nil, err
	}
	if result.Code != 0 {
		log.Warnw("code not 0",
			"code", result.Code, "message", result.Message, "request_id", result.RequestID)
		return nil, fmt.Errorf("code:%d message:%s request_id:%s", result.Code, result.Message, result.RequestID)
	}
	return result.Data, nil
}

// Convert2Pdf 文档转换成pdf格式（直接使用本地 soffice/unoconv，不再调用私有转换服务）
func (s *svcImpl) Convert2Pdf(c *gin.Context, srcFilename, targetFilename string) (err error) {
	return s.Convert2PdfBySoffice(c, srcFilename, targetFilename)
}

func IsPDFFileValid(filename string) (bool, error) {
	f, err := os.Open(filename)
	if err != nil {
		return false, err
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return false, err
	}
	hdr := make([]byte, 5)
	if _, err := io.ReadFull(f, hdr); err != nil {
		return false, fmt.Errorf("read header failed: %w", err)
	}
	if string(hdr) != "%PDF-" {
		return false, fmt.Errorf("missing %s header", "%PDF")
	}
	tailSize := int64(8192)
	if stat.Size() < tailSize {
		tailSize = stat.Size()
	}
	bufLen := int(tailSize)
	if bufLen <= 0 {
		bufLen = 1
	}
	tail := make([]byte, bufLen)
	_, err = f.ReadAt(tail, stat.Size()-int64(bufLen))
	if err != nil {
		// 尾部读取失败不作为致命错误，仍可按头部有效判定
		return true, nil
	}
	if !strings.Contains(string(tail), "%%EOF") {
		// 部分PDF可能不包含明确的EOF标记，按头部有效继续
		return true, nil
	}
	return true, nil
}

func toLogSnippet(contentType string, body []byte, limit int) string {
	ct := strings.ToLower(strings.TrimSpace(contentType))
	if strings.Contains(ct, "application/json") || strings.HasPrefix(ct, "text/") {
		s := string(body)
		if len(s) > limit {
			s = s[:limit]
		}
		return s
	}
	return fmt.Sprintf("binary(%d bytes)", len(body))
}

func (s *svcImpl) Convert2PdfCompat(c *gin.Context, srcFilename, targetFilename string) error {
	log := s.logger.With(entity.Ctx(c)...)
	if err := s.Convert2Pdf(c, srcFilename, targetFilename); err == nil {
		ok, _ := IsPDFFileValid(targetFilename)
		if ok {
			return nil
		}
		log.Warnw("convert2pdf生成的PDF无效，回退soffice", "targetFilename", targetFilename)
	} else {
		log.Warnw("convert2pdf调用失败，回退soffice", "err", err, "src", srcFilename)
	}
	if p, _ := exec.LookPath("soffice"); p != "" {
		outdir := os.TempDir()
		base := filepath.Base(srcFilename)
		ext := strings.ToLower(filepath.Ext(base))
		filter := "pdf:writer_pdf_Export"
		if ext == ".ppt" || ext == ".pptx" {
			filter = "pdf:impress_pdf_Export"
		}
		cmd := exec.Command(p, "--headless", "--norestore", "--nolockcheck", "--convert-to", filter, "--outdir", outdir, srcFilename)
		cmd.Env = os.Environ()
		out, err := cmd.CombinedOutput()
		if err != nil {
			log.Warnw("soffice转换失败", "err", err, "outdir", outdir, "output", toLogSnippet("text/plain", out, 512))
		} else {
			name := strings.TrimSuffix(base, ext)
			srcPdf := filepath.Join(outdir, name+".pdf")
			if fi, e := os.Stat(srcPdf); e == nil && fi.Size() > 0 {
				if e2 := copyFile(srcPdf, targetFilename); e2 == nil {
					ok, _ := IsPDFFileValid(targetFilename)
					if ok {
						log.Infow("soffice转换成功", "src", srcFilename, "dst", targetFilename, "size", fi.Size())
						return nil
					} else {
						log.Warnw("soffice生成的PDF无效", "dst", targetFilename)
					}
				} else {
					log.Warnw("复制soffice输出失败", "srcPdf", srcPdf, "dst", targetFilename, "err", e2)
				}
			} else {
				log.Warnw("soffice未生成输出文件", "srcPdf", srcPdf, "stat_err", e)
			}
		}
	} else {
		log.Warnw("未找到soffice可执行文件")
	}
	if p, _ := exec.LookPath("unoconv"); p != "" {
		cmd := exec.Command(p, "-f", "pdf", "-o", targetFilename, srcFilename)
		cmd.Env = os.Environ()
		out, err := cmd.CombinedOutput()
		if err != nil {
			log.Warnw("unoconv转换失败", "err", err, "output", toLogSnippet("text/plain", out, 512))
		} else {
			ok, _ := IsPDFFileValid(targetFilename)
			if ok {
				log.Infow("unoconv转换成功", "src", srcFilename, "dst", targetFilename)
				return nil
			} else {
				log.Warnw("unoconv生成的PDF无效", "dst", targetFilename)
			}
		}
	} else {
		log.Warnw("未找到unoconv可执行文件")
	}
	return fmt.Errorf("convert2pdf compat failed")
}

func (s *svcImpl) Convert2PdfBySoffice(c *gin.Context, srcFilename, targetFilename string) error {
	log := s.logger.With(entity.Ctx(c)...)
	// 转换能力已拆到独立的 doc-converter 依赖服务（见 docker-compose.yml），
	// 后端镜像不再内置 LibreOffice。
	tmpPDF, err := converter.GetInstance().ConvertToPDF(entity.ConvertContext(c), srcFilename)
	if err != nil {
		log.Warnw("文档转换服务调用失败", "src", srcFilename, "err", err)
		return err
	}
	defer func() { _ = os.Remove(tmpPDF) }()
	if err := copyFile(tmpPDF, targetFilename); err != nil {
		log.Warnw("复制转换结果失败", "src", tmpPDF, "dst", targetFilename, "err", err)
		return err
	}
	if ok, _ := IsPDFFileValid(targetFilename); ok {
		log.Infow("文档转换成功", "src", srcFilename, "dst", targetFilename)
		return nil
	}
	log.Warnw("转换得到的PDF无效", "dst", targetFilename)
	return fmt.Errorf("文档转换服务返回的 PDF 无效")
}

// convert2PdfByLocalSoffice 仅用于本机调试（当前运行镜像不含 LibreOffice）
func (s *svcImpl) convert2PdfByLocalSoffice(c *gin.Context, srcFilename, targetFilename string) error {
	log := s.logger.With(entity.Ctx(c)...)
	if p, _ := exec.LookPath("soffice"); p != "" {
		outdir := os.TempDir()
		base := filepath.Base(srcFilename)
		ext := strings.ToLower(filepath.Ext(base))
		filter := "pdf:writer_pdf_Export"
		if ext == ".ppt" || ext == ".pptx" {
			filter = "pdf:impress_pdf_Export"
		}
		cmd := exec.Command(p, "--headless", "--norestore", "--nolockcheck", "--convert-to", filter, "--outdir", outdir, srcFilename)
		cmd.Env = os.Environ()
		out, err := cmd.CombinedOutput()
		if err != nil {
			log.Warnw("soffice转换失败", "err", err, "outdir", outdir, "output", toLogSnippet("text/plain", out, 512))
		} else {
			name := strings.TrimSuffix(base, ext)
			srcPdf := filepath.Join(outdir, name+".pdf")
			if fi, e := os.Stat(srcPdf); e == nil && fi.Size() > 0 {
				if e2 := copyFile(srcPdf, targetFilename); e2 == nil {
					ok, _ := IsPDFFileValid(targetFilename)
					if ok {
						log.Infow("soffice转换成功", "src", srcFilename, "dst", targetFilename, "size", fi.Size())
						return nil
					} else {
						log.Warnw("soffice生成的PDF无效", "dst", targetFilename)
					}
				} else {
					log.Warnw("复制soffice输出失败", "srcPdf", srcPdf, "dst", targetFilename, "err", e2)
				}
			} else {
				log.Warnw("soffice未生成输出文件", "srcPdf", srcPdf, "stat_err", e)
			}
		}
	} else {
		log.Warnw("未找到soffice可执行文件")
	}
	if p, _ := exec.LookPath("unoconv"); p != "" {
		cmd := exec.Command(p, "-f", "pdf", "-o", targetFilename, srcFilename)
		cmd.Env = os.Environ()
		out, err := cmd.CombinedOutput()
		if err != nil {
			log.Warnw("unoconv转换失败", "err", err, "output", toLogSnippet("text/plain", out, 512))
		} else {
			ok, _ := IsPDFFileValid(targetFilename)
			if ok {
				log.Infow("unoconv转换成功", "src", srcFilename, "dst", targetFilename)
				return nil
			} else {
				log.Warnw("unoconv生成的PDF无效", "dst", targetFilename)
			}
		}
	} else {
		log.Warnw("未找到unoconv可执行文件")
	}
	return fmt.Errorf("convert2pdf by soffice failed")
}

func (s *svcImpl) ConvertDocToDocx(c *gin.Context, srcFilename, targetFilename string) error {
	log := s.logger.With(entity.Ctx(c)...)
	if p, _ := exec.LookPath("soffice"); p != "" {
		outdir := os.TempDir()
		base := filepath.Base(srcFilename)
		ext := strings.ToLower(filepath.Ext(base))
		cmd := exec.Command(p, "--headless", "--norestore", "--nolockcheck", "--convert-to", "docx:MS Word 2007 XML", "--outdir", outdir, srcFilename)
		cmd.Env = os.Environ()
		out, err := cmd.CombinedOutput()
		if err != nil {
			log.Warnw("soffice转换doc到docx失败", "err", err, "outdir", outdir, "output", toLogSnippet("text/plain", out, 512))
		} else {
			name := strings.TrimSuffix(base, ext)
			srcDocx := filepath.Join(outdir, name+".docx")
			if fi, e := os.Stat(srcDocx); e == nil && fi.Size() > 0 {
				if e2 := copyFile(srcDocx, targetFilename); e2 == nil {
					log.Infow("soffice转换doc到docx成功", "src", srcFilename, "dst", targetFilename, "size", fi.Size())
					return nil
				} else {
					log.Warnw("复制soffice输出失败", "srcDocx", srcDocx, "dst", targetFilename, "err", e2)
				}
			} else {
				log.Warnw("soffice未生成输出文件", "srcDocx", srcDocx, "stat_err", e)
			}
		}
	} else {
		log.Warnw("未找到soffice可执行文件")
	}
	if p, _ := exec.LookPath("unoconv"); p != "" {
		cmd := exec.Command(p, "-f", "docx", "-o", targetFilename, srcFilename)
		cmd.Env = os.Environ()
		out, err := cmd.CombinedOutput()
		if err != nil {
			log.Warnw("unoconv转换doc到docx失败", "err", err, "output", toLogSnippet("text/plain", out, 512))
		} else {
			if fi, e := os.Stat(targetFilename); e == nil && fi.Size() > 0 {
				log.Infow("unoconv转换doc到docx成功", "src", srcFilename, "dst", targetFilename)
				return nil
			} else {
				log.Warnw("unoconv未生成输出文件", "dst", targetFilename, "stat_err", e)
			}
		}
	} else {
		log.Warnw("未找到unoconv可执行文件")
	}
	return fmt.Errorf("convert doc to docx failed")
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

type CharIndex struct {
	Char    string  `json:"char"`
	PageNum int64   `json:"pageNum"`
	Index   int     `json:"index"`
	Left    float64 `json:"left"`
	Right   float64 `json:"right"`
	Bottom  float64 `json:"bottom"`
	Top     float64 `json:"top"`
}

// CalculateCharIndex 根据PDF解析返回的Data结果，计算每个字符相对全文的页码、字符索引
func (s *svcImpl) CalculateCharIndex(c *gin.Context, data *Data) ([]CharIndex, error) {
	var charIndices []CharIndex
	index := 1
	if len(data.Pages) == 0 {
		return charIndices, nil
	}
	for _, page := range data.Pages {
		if page == nil || len(page.Blocks) == 0 {
			continue
		}
		for _, blk := range page.Blocks {
			if blk == nil || len(blk.Chars) == 0 {
				continue
			}
			runes := []rune(blk.Chars)
			for i := 0; i < len(runes); i++ {
				var l, rgt, btm, tp float64
				if i < len(blk.CharPositions) {
					pos := blk.CharPositions[i]
					if len(pos) >= 4 {
						// 溯源坐标语义：pos = [left, right, bottom, top]
						l = float64(pos[0])
						rgt = float64(pos[1])
						btm = float64(pos[2])
						tp = float64(pos[3])
					}
				}
				charIndices = append(charIndices, CharIndex{PageNum: page.PageIndex + 1, Char: string(runes[i]), Index: index, Left: l, Right: rgt, Bottom: btm, Top: tp})
				index++
			}
		}
	}
	return charIndices, nil
}
