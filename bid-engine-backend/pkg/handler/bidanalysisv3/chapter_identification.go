package bidanalysisv3

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"bid-engine/pkg/db/model"
)

var weakChapterTitleRe = regexp.MustCompile(`^(?:[0-9]+[、.．）)]?[\s　]+|[一二三四五六七八九十百零]+、)`)

type chapterBoundaryPlan struct {
	Boundaries     []chapterBoundary
	NeedsLLMRefine bool
	UsedFallback   bool
}

func planChapterBoundaries(blocks []*model.BidAnalysisV3DocumentBlock) chapterBoundaryPlan {
	strong := collectChapterBoundaries(blocks, func(text string) bool { return topChapterTitleRe.MatchString(text) })
	if len(strong) > 0 {
		for i := range strong {
			strong[i].Type = classifyChapter(strong[i].Title)
		}
		return chapterBoundaryPlan{Boundaries: prependFrontMatter(blocks, strong)}
	}

	weak := collectChapterBoundaries(blocks, func(text string) bool { return weakChapterTitleRe.MatchString(text) })
	if len(weak) >= 2 {
		return chapterBoundaryPlan{Boundaries: prependFrontMatter(blocks, weak), NeedsLLMRefine: true}
	}

	boundaries := make([]chapterBoundary, 0, len(blocks)/20+1)
	for start := 0; start < len(blocks); {
		pageStart := blocks[start].PageNo
		pageEnd := pageStart + 19
		end := start + 1
		for end < len(blocks) && blocks[end].PageNo <= pageEnd {
			end++
		}
		boundaries = append(boundaries, chapterBoundary{Index: start, Title: fmt.Sprintf("第 %d–%d 页", pageStart, blocks[end-1].PageNo), Page: pageStart, Type: "other"})
		start = end
	}
	return chapterBoundaryPlan{Boundaries: boundaries, UsedFallback: true}
}

func collectChapterBoundaries(blocks []*model.BidAnalysisV3DocumentBlock, match func(string) bool) []chapterBoundary {
	out := make([]chapterBoundary, 0, 16)
	for index, block := range blocks {
		text := strings.TrimSpace(block.Text)
		if text == "" || utf8.RuneCountInString(text) > 100 || !match(text) || isSentenceLikeHeading(text) || isChapterNoise(text) {
			continue
		}
		out = append(out, chapterBoundary{Index: index, Title: text, Page: block.PageNo})
	}
	return out
}

func prependFrontMatter(blocks []*model.BidAnalysisV3DocumentBlock, boundaries []chapterBoundary) []chapterBoundary {
	if len(blocks) == 0 || len(boundaries) == 0 || boundaries[0].Index == 0 {
		return boundaries
	}
	front := chapterBoundary{Index: 0, Title: "封面与前置内容", Page: blocks[0].PageNo, Type: "other"}
	return append([]chapterBoundary{front}, boundaries...)
}
