package bidgen

import (
	"context"
	"fmt"
	"sort"

	"bid-engine/pkg/db/model"
	"bid-engine/pkg/service/biddoc"
)

// topUpShortfallChapterLimit 一轮补齐最多处理的章节数（控制 LLM 成本与任务时长）
const topUpShortfallChapterLimit = 6

// topUpGenerationShortfall 篇幅达标机制：全篇字数低于档位下限时，
// 挑出字数缺口最大的若干章节做一轮定向扩写。只允许补充论证、举证、
// 流程与细节（扩写模式的既有约束），不无限重试。
func (s *svcImpl) topUpGenerationShortfall(ctx context.Context, proj *model.BidGenProject, task *model.BidGenTask, ordered []*model.BidGenOutline, wordTargets map[int64]int) error {
	if proj == nil || task == nil {
		return nil
	}
	minimum := fullDocMinWords(task.LengthTier)
	total, actualByOutline, err := s.projectWordCount(ctx, proj.ID)
	if err != nil {
		return err
	}
	if total >= minimum {
		return nil
	}

	type chapterDeficit struct {
		node   *model.BidGenOutline
		target int
		actual int
		gap    int
	}
	deficits := make([]chapterDeficit, 0, len(ordered))
	for _, node := range ordered {
		if biddoc.IsDocumentRoot(node) {
			continue
		}
		target, ok := wordTargets[node.ID]
		if !ok || target <= 0 {
			continue
		}
		actual := actualByOutline[node.ID]
		deficits = append(deficits, chapterDeficit{node: node, target: target, actual: actual, gap: target - actual})
	}
	sort.SliceStable(deficits, func(i, j int) bool {
		if deficits[i].gap != deficits[j].gap {
			return deficits[i].gap > deficits[j].gap
		}
		return deficits[i].node.ID < deficits[j].node.ID
	})

	globalCtx := s.buildGlobalContext(ctx, proj)
	processed := 0
	for _, deficit := range deficits {
		if total >= minimum || processed >= topUpShortfallChapterLimit {
			break
		}
		if deficit.gap <= 100 {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		instruction := fmt.Sprintf("本章当前约 %d 字，请扩写至约 %d 字；只补充论证、举证、实施流程、保障机制与细节，不得注水、不得新增原文没有的事实。",
			deficit.actual, deficit.target)
		s.logger.Infow("标书篇幅不足，定向扩写章节",
			"project_id", proj.ID, "task_id", task.ID, "outline_id", deficit.node.ID,
			"total_words", total, "minimum", minimum)
		if _, err := s.generateChapter(ctx, proj, deficit.node, ordered, wordTargets, globalCtx,
			task.LengthTier, GenModeExpand, instruction, task.ID, int(task.CompletedCount), int(task.TotalCount)); err != nil {
			return err
		}
		processed++
		if next, _, err := s.projectWordCount(ctx, proj.ID); err == nil {
			total = next
		}
	}
	if total < minimum {
		s.logger.Warnw("标书篇幅不足且已达补齐上限",
			"project_id", proj.ID, "task_id", task.ID, "total_words", total, "minimum", minimum)
	}
	return nil
}

// projectWordCount 统计全篇正文字数（按章节落库的 word_count 汇总）。
func (s *svcImpl) projectWordCount(ctx context.Context, projectID int64) (int, map[int64]int, error) {
	contents, err := s.repo.GetChapterContentsByProjectID(ctx, projectID)
	if err != nil {
		return 0, nil, err
	}
	actual := make(map[int64]int, len(contents))
	total := 0
	for _, content := range contents {
		if content == nil {
			continue
		}
		actual[content.OutlineID] = int(content.WordCount)
		total += int(content.WordCount)
	}
	return total, actual, nil
}
