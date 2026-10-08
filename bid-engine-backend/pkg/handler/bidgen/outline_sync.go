package bidgen

import (
	"strings"

	"bid-engine/pkg/entity"
)

// deriveOutlineTree 将文档顺序的标题列表推导为大纲树扁平项。
// - parent 以数组下标引用（-1=顶层），由调用方在节点落库后解析为真实 parent_id；
// - sort_order = 同级序号 * 100，保证文档顺序即导航顺序；
// - level 夹取到 1-4（与大纲表约定一致）。
func deriveOutlineTree(headings []entity.BidGenSyncHeading) []entity.BidGenOutlineSyncItem {
	type frame struct {
		level int32
		idx   int64
	}
	items := make([]entity.BidGenOutlineSyncItem, 0, len(headings))
	stack := make([]frame, 0, 8)
	siblingCount := make(map[int64]int)
	for i, h := range headings {
		level := h.Level
		if level < 1 {
			level = 1
		}
		if level > 4 {
			level = 4
		}
		for len(stack) > 0 && stack[len(stack)-1].level >= level {
			stack = stack[:len(stack)-1]
		}
		parentIdx := int64(-1)
		if len(stack) > 0 {
			parentIdx = stack[len(stack)-1].idx
		}
		ord := siblingCount[parentIdx]
		siblingCount[parentIdx] = ord + 1
		items = append(items, entity.BidGenOutlineSyncItem{
			Index:       int64(i),
			OutlineID:   h.OutlineID,
			ParentIndex: parentIdx,
			Level:       level,
			Title:       strings.TrimSpace(h.Title),
			SortOrder:   int32(ord) * 100,
		})
		stack = append(stack, frame{level: level, idx: int64(i)})
	}
	return items
}
