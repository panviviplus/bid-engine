package biddoc

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"bid-engine/pkg/db/model"
)

type AnchorState string

const (
	AnchorComplete AnchorState = "complete"
	AnchorEmpty    AnchorState = "empty"
	AnchorPartial  AnchorState = "partial"
)

type AnchorInspection struct {
	State   AnchorState
	Found   []int64
	Missing []int64
}

type pmNode struct {
	Type    string                 `json:"type"`
	Attrs   map[string]interface{} `json:"attrs,omitempty"`
	Content []pmNode               `json:"content,omitempty"`
	Text    string                 `json:"text,omitempty"`
}

// OrderOutlineTree returns a stable parent-first traversal. SortOrder is scoped
// to siblings, so a database-wide ORDER BY sort_order is not a document order.
func OrderOutlineTree(nodes []*model.BidGenOutline) ([]*model.BidGenOutline, error) {
	if len(nodes) == 0 {
		return []*model.BidGenOutline{}, nil
	}
	byID := make(map[int64]*model.BidGenOutline, len(nodes))
	children := make(map[int64][]*model.BidGenOutline, len(nodes))
	for _, node := range nodes {
		if node == nil || node.ID <= 0 {
			return nil, fmt.Errorf("大纲包含无效节点")
		}
		if node.Level < 1 || node.Level > 6 {
			return nil, fmt.Errorf("大纲节点 %d 的层级无效: %d", node.ID, node.Level)
		}
		if strings.TrimSpace(node.Title) == "" {
			return nil, fmt.Errorf("大纲节点 %d 的标题为空", node.ID)
		}
		if _, exists := byID[node.ID]; exists {
			return nil, fmt.Errorf("大纲节点 ID 重复: %d", node.ID)
		}
		byID[node.ID] = node
	}
	for _, node := range nodes {
		if node.ParentID != 0 {
			if _, exists := byID[node.ParentID]; !exists {
				return nil, fmt.Errorf("大纲节点 %d 的父节点 %d 不存在", node.ID, node.ParentID)
			}
		}
		children[node.ParentID] = append(children[node.ParentID], node)
	}
	less := func(items []*model.BidGenOutline) {
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].SortOrder == items[j].SortOrder {
				return items[i].ID < items[j].ID
			}
			return items[i].SortOrder < items[j].SortOrder
		})
	}
	for parentID := range children {
		less(children[parentID])
	}

	ordered := make([]*model.BidGenOutline, 0, len(nodes))
	visiting := make(map[int64]bool, len(nodes))
	visited := make(map[int64]bool, len(nodes))
	var walk func(*model.BidGenOutline) error
	walk = func(node *model.BidGenOutline) error {
		if visiting[node.ID] {
			return fmt.Errorf("大纲存在循环引用，节点 ID: %d", node.ID)
		}
		if visited[node.ID] {
			return nil
		}
		visiting[node.ID] = true
		ordered = append(ordered, node)
		for _, child := range children[node.ID] {
			if err := walk(child); err != nil {
				return err
			}
		}
		visiting[node.ID] = false
		visited[node.ID] = true
		return nil
	}
	for _, root := range children[0] {
		if err := walk(root); err != nil {
			return nil, err
		}
	}
	if len(ordered) != len(nodes) {
		return nil, fmt.Errorf("大纲结构不完整或存在循环引用")
	}
	return ordered, nil
}

func BuildOutlineDocument(nodes []*model.BidGenOutline) (string, error) {
	ordered, err := OrderOutlineTree(nodes)
	if err != nil {
		return "", err
	}
	doc := pmNode{Type: "doc", Content: []pmNode{{Type: "paragraph"}}}
	for _, node := range ordered {
		doc.Content = append(doc.Content,
			pmNode{
				Type:    "heading",
				Attrs:   map[string]interface{}{"level": int(node.Level), "outlineId": node.ID},
				Content: []pmNode{{Type: "text", Text: node.Title}},
			},
			pmNode{Type: "paragraph"},
		)
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("序列化大纲文档: %w", err)
	}
	return string(raw), nil
}

func InspectOutlineAnchors(docJSON string, nodes []*model.BidGenOutline) (AnchorInspection, error) {
	ordered, err := OrderOutlineTree(nodes)
	if err != nil {
		return AnchorInspection{}, err
	}
	var doc pmNode
	if err := json.Unmarshal([]byte(docJSON), &doc); err != nil {
		return AnchorInspection{}, fmt.Errorf("解析正文结构: %w", err)
	}
	if doc.Type != "doc" {
		return AnchorInspection{}, fmt.Errorf("正文根节点类型无效: %s", doc.Type)
	}
	expected := make(map[int64]struct{}, len(ordered))
	for _, node := range ordered {
		expected[node.ID] = struct{}{}
	}
	foundSet := make(map[int64]struct{}, len(ordered))
	var collect func(pmNode)
	collect = func(node pmNode) {
		if node.Type == "heading" {
			if id, ok := outlineID(node.Attrs["outlineId"]); ok {
				if _, wanted := expected[id]; wanted {
					foundSet[id] = struct{}{}
				}
			}
		}
		for _, child := range node.Content {
			collect(child)
		}
	}
	collect(doc)

	inspection := AnchorInspection{}
	for _, node := range ordered {
		if _, ok := foundSet[node.ID]; ok {
			inspection.Found = append(inspection.Found, node.ID)
		} else {
			inspection.Missing = append(inspection.Missing, node.ID)
		}
	}
	switch {
	case len(inspection.Missing) == 0:
		inspection.State = AnchorComplete
	case len(inspection.Found) == 0:
		inspection.State = AnchorEmpty
	default:
		inspection.State = AnchorPartial
	}
	return inspection, nil
}

func IsDocumentRoot(node *model.BidGenOutline) bool {
	return node != nil && node.ParentID == 0 && node.Source == "system_root"
}

func outlineID(value interface{}) (int64, bool) {
	switch id := value.(type) {
	case float64:
		return int64(id), id > 0
	case int64:
		return id, id > 0
	case json.Number:
		parsed, err := id.Int64()
		return parsed, err == nil && parsed > 0
	case string:
		parsed, err := strconv.ParseInt(id, 10, 64)
		return parsed, err == nil && parsed > 0
	default:
		return 0, false
	}
}
