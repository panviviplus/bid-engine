package bidanalysisv3

import (
	"regexp"
	"strings"

	"bid-engine/pkg/db/model"
)

// topChapterTitleRe 识别招标文件的顶级大章节标题形态：
// “第一章/第二章…”（第X章）、第X部（分）、第X篇，以及“附件X”。
// 小标题（“1 适用范围”“一、总则”“6、投标文件的递交”等）不会被识别为顶级章节；
// “第X节”（如“第一节专用合同条款”）通常是某章的子级，也不作为顶级。
var topChapterTitleRe = regexp.MustCompile(`^(第[一二三四五六七八九十百零0-9]+[章部篇]|附件[一二三四五六七八九十0-9]*)`)

// topChapterGroup 一个顶级章节分组：顶级章节 + 其页码区间 + 区间内的关键条款。
type topChapterGroup struct {
	Chapter   *model.BidAnalysisV3Chapter
	PageStart int
	PageEnd   int
	Clauses   []*model.BidAnalysisV3Clause
}

// buildTopChapterGroups 将细粒度章节按“顶级大章节”重新分组：
//   - 顶级章节 = 首个章节（文档前置内容，如“封面与前置内容”）+ 标题符合 topChapterTitleRe 的章节；
//   - 每个顶级章节的页码区间为 [自身起始页, 下一个顶级章节起始页-1]，末组延伸到 maxPage；
//   - 条款按其所属章节的起始页归入对应顶级组；
//   - 只返回提炼到关键条款的顶级组（没有条款的 group 不返回，前端无需再过滤空组）。
//
// 若规则识别不出顶级章节（或全部章节都被判为顶级），回退为按全部章节分组，保证 tab 仍可用。
func buildTopChapterGroups(chapters []*model.BidAnalysisV3Chapter, clauses []*model.BidAnalysisV3Clause, maxPage int) []topChapterGroup {
	if len(chapters) == 0 {
		return nil
	}
	tops := make([]*model.BidAnalysisV3Chapter, 0, 8)
	matchedTopTitle := false
	for i, ch := range chapters {
		isTopTitle := topChapterTitleRe.MatchString(strings.TrimSpace(ch.ChapterTitle))
		if isTopTitle && i > 0 {
			matchedTopTitle = true
		}
		if i == 0 || isTopTitle {
			tops = append(tops, ch)
		}
	}
	// 规则没有识别出任何“第X章/部分/篇/附件”形态的顶级标题时，
	// 说明文档大纲不以该形态组织，回退为按全部章节分组，保证 tab 仍可用。
	if !matchedTopTitle || len(tops) == len(chapters) {
		tops = chapters
	}
	spans := make([][2]int, len(tops))
	for i, top := range tops {
		end := maxPage
		if i+1 < len(tops) {
			end = int(tops[i+1].PageStart) - 1
		}
		spans[i] = [2]int{int(top.PageStart), end}
	}
	groups := make([]topChapterGroup, len(tops))
	for i, top := range tops {
		groups[i] = topChapterGroup{Chapter: top, PageStart: int(top.PageStart), PageEnd: spans[i][1]}
	}
	byChapterID := make(map[int64]*model.BidAnalysisV3Chapter, len(chapters))
	for _, ch := range chapters {
		byChapterID[ch.ID] = ch
	}
	for _, cl := range clauses {
		host := byChapterID[cl.ChapterID]
		if host == nil {
			continue
		}
		idx := spanIndexForPage(spans, int(host.PageStart))
		if idx < 0 {
			idx = 0
		}
		groups[idx].Clauses = append(groups[idx].Clauses, cl)
	}
	out := groups[:0]
	for _, g := range groups {
		if len(g.Clauses) > 0 {
			out = append(out, g)
		}
	}
	return out
}

func spanIndexForPage(spans [][2]int, page int) int {
	for i, s := range spans {
		if page >= s[0] && page <= s[1] {
			return i
		}
	}
	return -1
}
