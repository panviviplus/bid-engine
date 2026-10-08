package bidgen

import (
	"encoding/json"

	"github.com/gin-gonic/gin"

	"bid-engine/pkg/db/model"
)

// insertHeadingToDoc 在文档中追加标题节点（大纲新增节点时同步）
func (s *svcImpl) insertHeadingToDoc(c *gin.Context, projectID int64, node *model.BidGenOutline) {
	dc, err := s.repo.GetDocContent(c, projectID)
	if err != nil || dc == nil || dc.DocJSON == "" {
		return
	}
	var doc pmNode
	if err := json.Unmarshal([]byte(dc.DocJSON), &doc); err != nil {
		return
	}
	doc.Content = append(doc.Content, headingNode(int(node.Level), node.ID, node.Title))
	doc.Content = append(doc.Content, paragraphNode())
	b, _ := json.Marshal(doc)
	_ = s.repo.UpsertDocContent(c, &model.BidGenDocContent{ProjectID: projectID, DocJSON: string(b), DocHTML: dc.DocHTML})
}

// updateHeadingInDoc 更新文档中对应标题节点文本
func (s *svcImpl) updateHeadingInDoc(c *gin.Context, projectID int64, node *model.BidGenOutline) {
	dc, err := s.repo.GetDocContent(c, projectID)
	if err != nil || dc == nil || dc.DocJSON == "" {
		return
	}
	var doc pmNode
	if err := json.Unmarshal([]byte(dc.DocJSON), &doc); err != nil {
		return
	}
	for i := range doc.Content {
		if doc.Content[i].Type != "heading" {
			continue
		}
		if v, ok := doc.Content[i].Attrs["outlineId"]; ok {
			if id, ok := v.(float64); ok && int64(id) == node.ID {
				doc.Content[i].Content = []pmNode{textNode(node.Title)}
				break
			}
		}
	}
	b, _ := json.Marshal(doc)
	_ = s.repo.UpsertDocContent(c, &model.BidGenDocContent{ProjectID: projectID, DocJSON: string(b), DocHTML: dc.DocHTML})
}

// deleteChapterRangeInDoc 删除文档中对应标题章节区间（含子级标题）
func (s *svcImpl) deleteChapterRangeInDoc(c *gin.Context, projectID int64, outlineID int64) {
	dc, err := s.repo.GetDocContent(c, projectID)
	if err != nil || dc == nil || dc.DocJSON == "" {
		return
	}
	var doc pmNode
	if err := json.Unmarshal([]byte(dc.DocJSON), &doc); err != nil {
		return
	}
	start, end := chapterRange(&doc, outlineID)
	if start < 0 {
		return
	}
	doc.Content = append(doc.Content[:start], doc.Content[end:]...)
	if len(doc.Content) == 0 {
		doc.Content = []pmNode{paragraphNode()}
	}
	b, _ := json.Marshal(doc)
	_ = s.repo.UpsertDocContent(c, &model.BidGenDocContent{ProjectID: projectID, DocJSON: string(b), DocHTML: dc.DocHTML})
}
