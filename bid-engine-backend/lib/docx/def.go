package docx

import (
	"fmt"
	"strings"

	"github.com/google/uuid"

	"bid-engine/pkg/entity"
)

type HeadingNode struct {
	Level    int
	Title    string
	Children []*HeadingNode
}

// 添加子节点到正确的父节点中
func insertHeading(root *HeadingNode, newNode *HeadingNode) {
	current := root
	for {
		if len(current.Children) == 0 || current.Children[len(current.Children)-1].Level >= newNode.Level {
			current.Children = append(current.Children, newNode)
			return
		}
		current = current.Children[len(current.Children)-1]
	}
}

// 打印目录结构
func printTree(node *HeadingNode, indent int) {
	if node.Title != "" {
		fmt.Printf("%s- %s\n", strings.Repeat("  ", indent), node.Title)
	}
	for _, child := range node.Children {
		printTree(child, indent+1)
	}
}

func (h *HeadingNode) toTemplate() []entity.Paragraph {
	var result []entity.Paragraph
	for _, child := range h.Children {
		result = append(result, entity.Paragraph{
			Title: entity.Content{
				ID:      uuid.NewString(),
				Content: child.Title,
			},
			Desc: entity.Content{
				ID:      uuid.NewString(),
				Content: child.Title,
			},
			Children: child.toTemplate(),
		})
	}
	return result
}
