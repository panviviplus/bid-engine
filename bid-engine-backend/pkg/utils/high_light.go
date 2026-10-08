package utils

import "sort"

// CharIndex 字符索引结构
type CharIndex struct {
	Char    string `json:"char"`
	PageNum int64  `json:"pageNum"`
	Index   int    `json:"index"`
	// 以下坐标信息是计算高亮所必须的，数据源里必须有这些值
	Left   float64 `json:"left"`
	Right  float64 `json:"right"`
	Bottom float64 `json:"bottom"`
	Top    float64 `json:"top"`
}

// HighlightArea 返回的高亮矩形结果
type HighlightArea struct {
	PageIndex int64   `json:"page_index"`
	Top       float64 `json:"top"`
	Left      float64 `json:"left"`
	Width     float64 `json:"width"`
	Height    float64 `json:"height"`
}

// CalcHighlightAreaRow 计算单行内的矩形（辅助函数）
func CalcHighlightAreaRow(chars []CharIndex) HighlightArea {
	if len(chars) == 0 {
		return HighlightArea{}
	}

	firstChar := chars[0]
	lastChar := chars[len(chars)-1]

	return HighlightArea{
		PageIndex: firstChar.PageNum,
		Top:       firstChar.Top,
		Left:      firstChar.Left,
		Width:     lastChar.Right - firstChar.Left,
		Height:    firstChar.Bottom - firstChar.Top,
	}
}

// CalcHighlightAreas 主函数：输入一组带坐标的字符，输出一个或多个高亮矩形
func CalcHighlightAreas(chars []CharIndex) []HighlightArea {
    var results []HighlightArea
    if len(chars) == 0 {
        return results
    }
    // 保证同一行左右顺序正确，按全文字符索引升序排列
    sort.SliceStable(chars, func(i, j int) bool { return chars[i].Index < chars[j].Index })

    const epsilon = 1.0 // 容忍同一行内Top的微小浮动（像素坐标）
    var lastTop float64 = -10000
    var lastPage int64 = -1
    var currRow []CharIndex

    for _, c := range chars {
        // 跳过无效坐标字符
        if c.Left == 0 && c.Right == 0 && c.Top == 0 && c.Bottom == 0 {
            continue
        }
        if lastTop == -10000 {
            lastTop = c.Top
            lastPage = c.PageNum
        }
        if c.PageNum == lastPage && (c.Top == lastTop || (c.Top > lastTop-epsilon && c.Top < lastTop+epsilon)) {
            currRow = append(currRow, c)
        } else {
            if len(currRow) > 0 {
                results = append(results, CalcHighlightAreaRow(currRow))
            }
            currRow = []CharIndex{c}
            lastTop = c.Top
            lastPage = c.PageNum
        }
    }
    if len(currRow) > 0 {
        results = append(results, CalcHighlightAreaRow(currRow))
    }
    return results
}
