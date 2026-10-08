package bidgen

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"bid-engine/pkg/db/model"
	"bid-engine/pkg/service/biddoc"
)

var errDocumentAnchorsPartial = errors.New("正文中的大纲标题不完整，请重新加载并确认大纲")

// RepairProjectDocument exposes the same guarded repair used by confirmation
// and generation to the maintenance command.
func RepairProjectDocument(ctx context.Context, projectID int64) error {
	svc, ok := GetInstance().(*svcImpl)
	if !ok {
		return fmt.Errorf("标书生成服务不可用")
	}
	nodes, err := svc.repo.GetOutlineByProjectID(ctx, projectID)
	if err != nil {
		return err
	}
	_, err = svc.ensureDocumentAnchors(ctx, projectID, nodes)
	return err
}

// ensureDocumentAnchors validates the outlineId anchors used by both the SSE
// editor replacement and the server-side merge. A document with zero anchors
// is safe to rebuild; a partially anchored document may contain user-authored
// content and is therefore rejected instead of being overwritten.
func (s *svcImpl) ensureDocumentAnchors(ctx context.Context, projectID int64, nodes []*model.BidGenOutline) (*model.BidGenDocContent, error) {
	ordered, err := biddoc.OrderOutlineTree(nodes)
	if err != nil {
		return nil, err
	}
	if len(ordered) == 0 {
		return nil, fmt.Errorf("大纲至少需要一个章节")
	}

	doc, err := s.repo.GetDocContent(ctx, projectID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("加载正文: %w", err)
	}
	if doc == nil || strings.TrimSpace(doc.DocJSON) == "" {
		return s.rebuildDocumentFromChapters(ctx, projectID, ordered, doc)
	}
	inspection, err := biddoc.InspectOutlineAnchors(doc.DocJSON, ordered)
	if err != nil {
		return nil, err
	}
	switch inspection.State {
	case biddoc.AnchorComplete:
		return doc, nil
	case biddoc.AnchorEmpty:
		return s.rebuildDocumentFromChapters(ctx, projectID, ordered, doc)
	default:
		return nil, fmt.Errorf("%w（缺少 %d 个标题锚点）", errDocumentAnchorsPartial, len(inspection.Missing))
	}
}

func (s *svcImpl) rebuildDocumentFromChapters(ctx context.Context, projectID int64, ordered []*model.BidGenOutline, previous *model.BidGenDocContent) (*model.BidGenDocContent, error) {
	docJSON, err := biddoc.BuildOutlineDocument(ordered)
	if err != nil {
		return nil, err
	}
	chapters, err := s.repo.GetChapterContentsByProjectID(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("加载已生成章节: %w", err)
	}
	byOutline := make(map[int64]*model.BidGenChapterContent, len(chapters))
	for _, chapter := range chapters {
		byOutline[chapter.OutlineID] = chapter
	}
	for _, outline := range ordered {
		chapter := byOutline[outline.ID]
		if chapter == nil || strings.TrimSpace(chapter.ContentJSON) == "" {
			continue
		}
		var segment []pmNode
		if err := json.Unmarshal([]byte(chapter.ContentJSON), &segment); err != nil {
			return nil, fmt.Errorf("解析章节 %d 的存量正文: %w", outline.ID, err)
		}
		docJSON, err = mergeChapter(docJSON, segment)
		if err != nil {
			return nil, fmt.Errorf("恢复章节 %d 到主文档: %w", outline.ID, err)
		}
	}
	docHTML := ""
	if previous != nil {
		docHTML = previous.DocHTML
	}
	rebuilt := &model.BidGenDocContent{ProjectID: projectID, DocJSON: docJSON, DocHTML: docHTML}
	if err := s.repo.UpsertDocContent(ctx, rebuilt); err != nil {
		return nil, fmt.Errorf("保存修复后的正文: %w", err)
	}
	return rebuilt, nil
}
