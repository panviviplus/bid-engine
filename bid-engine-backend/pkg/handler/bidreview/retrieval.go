package bidreview

import (
	"context"
	"sort"
	"strings"

	"bid-engine/pkg/db/model"
)

// pageHit 召回命中的投标文件页（作为判定的证据候选）
type pageHit struct {
	FileID   int64
	FileName string
	PageNo   int32
	Text     string
	Score    float64
}

// itemQueryText 清单项的召回文本（判定口径 + 招投标术语）
func itemQueryText(item *model.BidReviewV2ChecklistItem) string {
	parts := []string{item.Title, item.Requirement, item.ExpectedEvidence, item.TenderQuote}
	return strings.Join(parts, "\n")
}

// recallForItem 为单个清单项召回候选页：
// 术语倒排（2/3-gram + 英文数字词）→ 命中分块 → 展开为页并按命中密度打分
func (s *svcImpl) recallForItem(ctx context.Context, projectID int64, item *model.BidReviewV2ChecklistItem, files map[int64]*model.BidReviewV2File) ([]pageHit, error) {
	terms := selectQueryTerms(itemQueryText(item), 18)
	if len(terms) == 0 {
		return nil, nil
	}
	topK := s.retrievalTopK
	if topK <= 0 {
		topK = 8
	}
	hits, err := s.repo.SearchTermIndex(ctx, projectID, terms, topK)
	if err != nil {
		return nil, err
	}
	if len(hits) == 0 {
		return nil, nil
	}

	termSet := make(map[string]struct{}, len(terms))
	for _, t := range terms {
		termSet[t] = struct{}{}
	}

	pageScores := make(map[string]*pageHit, len(hits)*2)
	for _, hit := range hits {
		pages, pErr := s.repo.GetPagesByFileRange(ctx, hit.FileID, hit.PageStart, hit.PageEnd)
		if pErr != nil {
			return nil, pErr
		}
		for _, p := range pages {
			text := strings.TrimSpace(p.Content)
			if text == "" {
				continue
			}
			score := hit.Score + float64(termOverlap(text, termSet))*2
			key := strings.Join([]string{itoa(hit.FileID), itoa(int64(p.PageNo))}, ":")
			name := ""
			if f, ok := files[hit.FileID]; ok && f != nil {
				name = f.FileName
			}
			if exist, ok := pageScores[key]; ok {
				if score > exist.Score {
					exist.Score = score
				}
				continue
			}
			pageScores[key] = &pageHit{
				FileID: hit.FileID, FileName: name, PageNo: p.PageNo,
				Text: truncateRunes(text, 1400), Score: score,
			}
		}
	}

	out := make([]pageHit, 0, len(pageScores))
	for _, v := range pageScores {
		out = append(out, *v)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if len(out) > topK {
		out = out[:topK]
	}
	return out, nil
}

// termOverlap 页面文本命中的查询词数量
func termOverlap(text string, terms map[string]struct{}) int {
	n := 0
	for t := range terms {
		if strings.Contains(text, t) {
			n++
		}
	}
	return n
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
