package docling

import (
	"sort"
	"strconv"

	pdfrepo "bid-engine/pkg/repo/pdf"
)

// =============================================================================
// PageText — 逐页文本（推荐新 Pipeline 使用）
// =============================================================================

// PageText 单页文本
type PageText struct {
	PageNum int
	Width   float64
	Height  float64
	Items   []PageTextItem
}

// PageTextItem 页内文本项
type PageTextItem struct {
	Text      string
	BBox      BBox
	Label     string // text / title / section_header / ...
	TableRefs []string
}

// =============================================================================
// ToPageTexts — DoclingDocument → 逐页文本
// =============================================================================

// ToPageTexts 将 DoclingDocument 转换为按页组织的文本列表。
// 自动过滤 content_layer="furniture" 的页眉/页脚，按 bbox 坐标排序还原阅读顺序。
func (d *DoclingDocument) ToPageTexts() []PageText {
	if d == nil {
		return nil
	}

	// 按页码分桶
	buckets := map[int][]TextItem{}
	for _, t := range d.Texts {
		if t.ContentLayer == "furniture" {
			continue // 跳过页眉/页脚
		}
		if len(t.Prov) == 0 {
			continue
		}
		pageNo := t.Prov[0].PageNo
		buckets[pageNo] = append(buckets[pageNo], t)
	}

	// 收集页码并排序
	pageNos := make([]int, 0, len(buckets))
	for p := range buckets {
		pageNos = append(pageNos, p)
	}
	sort.Ints(pageNos)

	result := make([]PageText, 0, len(pageNos))
	for _, pn := range pageNos {
		items := buckets[pn]

		// 页内按 t 坐标降序 → 还原从上到下的阅读顺序
		sort.SliceStable(items, func(i, j int) bool {
			return items[i].Prov[0].BBox.T > items[j].Prov[0].BBox.T
		})

		pt := PageText{
			PageNum: pn,
			Items:   make([]PageTextItem, 0, len(items)),
		}

		// 获取页面尺寸
		pageKey := strconv.Itoa(pn)
		if pi, ok := d.Pages[pageKey]; ok {
			pt.Width = pi.Size.Width
			pt.Height = pi.Size.Height
		}

		for _, item := range items {
			bbox := item.Prov[0].BBox
			pt.Items = append(pt.Items, PageTextItem{
				Text:  item.Text,
				BBox:  bbox,
				Label: item.Label,
			})
		}
		result = append(result, pt)
	}

	return result
}

// ToFullText 返回 DoclingDocument 中所有 body 文本拼接成的全文纯文本。
func (d *DoclingDocument) ToFullText() string {
	if d == nil {
		return ""
	}
	pts := d.ToPageTexts()
	var buf []byte
	for _, pt := range pts {
		for _, item := range pt.Items {
			buf = append(buf, item.Text...)
			buf = append(buf, '\n')
		}
	}
	return string(buf)
}

// =============================================================================
// ToPDFData — DoclingDocument → pdf.Data（向后兼容）
// =============================================================================

// ToPDFData 将 DoclingDocument 转换为旧 pdf.Data 格式。
// 用于渐进迁移：handler 无需改动即可使用 Docling。
func (d *DoclingDocument) ToPDFData() *pdfrepo.Data {
	if d == nil {
		return &pdfrepo.Data{Pages: []*pdfrepo.Page{}}
	}

	// 按页码分桶
	buckets := map[int][]TextItem{}
	for _, t := range d.Texts {
		if t.ContentLayer == "furniture" {
			continue
		}
		if len(t.Prov) == 0 {
			continue
		}
		pageNo := t.Prov[0].PageNo
		buckets[pageNo] = append(buckets[pageNo], t)
	}

	pageNos := make([]int, 0, len(buckets))
	for p := range buckets {
		pageNos = append(pageNos, p)
	}
	sort.Ints(pageNos)

	var (
		pages     []*pdfrepo.Page
		totalChar int64
	)

	for _, pn := range pageNos {
		items := buckets[pn]
		sort.SliceStable(items, func(i, j int) bool {
			return items[i].Prov[0].BBox.T > items[j].Prov[0].BBox.T
		})

		blocks := make([]*pdfrepo.Block, 0, len(items))
		var pageCharCount int64

		for bi, item := range items {
			bbox := item.Prov[0].BBox
			charCount := int64(len([]rune(item.Text)))

			block := &pdfrepo.Block{
				BlockIndex: int64(bi),
				CharCount:  charCount,
				Chars:      item.Text,
				// 段落级 bounding box — 精度从逐字符降为段落级
				CharPositions: [][]int64{
					{int64(bbox.L), int64(bbox.R), int64(bbox.B), int64(bbox.T)},
				},
			}
			blocks = append(blocks, block)
			pageCharCount += charCount
		}

		pageKey := strconv.Itoa(pn)
		width := int32(612)  // 默认 US Letter
		height := int32(792)
		if pi, ok := d.Pages[pageKey]; ok {
			width = int32(pi.Size.Width)
			height = int32(pi.Size.Height)
		}

		pages = append(pages, &pdfrepo.Page{
			BlockCount: int64(len(blocks)),
			Blocks:     blocks,
			CharCount:  pageCharCount,
			Height:     height,
			Width:      width,
			PageIndex:  int64(pn - 1),
		})
		totalChar += pageCharCount
	}

	return &pdfrepo.Data{
		Id:        d.Name,
		PageCount: int64(len(pages)),
		CharCount: totalChar,
		Pages:     pages,
	}
}

// =============================================================================
// ToCharIndices — DoclingDocument → 段落级溯源索引
// =============================================================================

// ToCharIndices 将 DoclingDocument 的文本项转换为段落级字符索引。
//
// 与 pdf.CalculateCharIndex 的区别：
//   - 旧：逐字符级别，每个字符一个 CharIndex 项
//   - 新：段落级别，每个 TextItem 一个 CharIndex 项，bbox 精度降为段落级
//
// 溯源高亮精度从"逐字精确"变为"段落级覆盖"，已确认可接受。
func (d *DoclingDocument) ToCharIndices() []pdfrepo.CharIndex {
	if d == nil {
		return nil
	}

	// 按页码分桶并排序
	buckets := map[int][]TextItem{}
	for _, t := range d.Texts {
		if t.ContentLayer == "furniture" {
			continue
		}
		if len(t.Prov) == 0 {
			continue
		}
		pageNo := t.Prov[0].PageNo
		buckets[pageNo] = append(buckets[pageNo], t)
	}

	pageNos := make([]int, 0, len(buckets))
	for p := range buckets {
		pageNos = append(pageNos, p)
	}
	sort.Ints(pageNos)

	var indices []pdfrepo.CharIndex
	globalIdx := 1

	for _, pn := range pageNos {
		items := buckets[pn]
		// 页内按 bbox.T 降序 → 还原阅读顺序
		sort.SliceStable(items, func(i, j int) bool {
			return items[i].Prov[0].BBox.T > items[j].Prov[0].BBox.T
		})

		for _, item := range items {
			bbox := normalizeBBox(item.Prov[0].BBox)
			// 对段落中每个字符都生成一个 CharIndex（保持与旧 API 兼容）
			runes := []rune(item.Text)
			for _, r := range runes {
				indices = append(indices, pdfrepo.CharIndex{
					Char:    string(r),
					PageNum: int64(pn),
					Index:   globalIdx,
					Left:    bbox.L,
					Right:   bbox.R,
					Bottom:  bbox.B,
					Top:     bbox.T,
				})
				globalIdx++
			}
		}
	}

	return indices
}

// =============================================================================
// normalizeBBox — 坐标原点归一化
// =============================================================================

// normalizeBBox 处理不同坐标原点 (BOTTOMLEFT vs TOPLEFT)。
// 当前仅透传 bbox 值，后续如需翻转 Y 轴可在此统一处理。
func normalizeBBox(b BBox) BBox {
	// Docling 的 bbox 默认使用 BOTTOMLEFT 原点。
	// 对于 PDF 显示（TOPLEFT 原点），需要翻转 Y 轴：
	//   new_t = page_height - b
	//   new_b = page_height - t
	// 由于 page_height 需要通过 pages map 查询，此处保持原始值。
	// 溯源高亮调用方可按需调用 normalizeBBoxForPage(b, pageHeight)。
	return b
}

// NormalizeBBoxForPage 将 BOTTOMLEFT 原点的 bbox 转换为 TOPLEFT 原点。
// 用于前端 PDF 预览高亮（前端使用 TOPLEFT 坐标系）。
func NormalizeBBoxForPage(b BBox, pageHeight float64) BBox {
	if b.CoordOrigin == "TOPLEFT" {
		return b
	}
	return BBox{
		L: b.L,
		T: pageHeight - b.B,
		R: b.R,
		B: pageHeight - b.T,
		CoordOrigin: "TOPLEFT",
	}
}

// =============================================================================
// ParseResult 便捷方法
// =============================================================================

// GetPageTexts 从 ParseResult 获取逐页文本（推荐新代码使用）。
// 内部调用 ParseDocument + ToPageTexts。
func (r *ParseResult) GetPageTexts() ([]PageText, error) {
	doc, err := r.ParseDocument()
	if err != nil {
		return nil, err
	}
	return doc.ToPageTexts(), nil
}

// GetFullText 从 ParseResult 获取全文纯文本。
func (r *ParseResult) GetFullText() (string, error) {
	return r.Text, nil // ParseResult.Text 已是全文
}
