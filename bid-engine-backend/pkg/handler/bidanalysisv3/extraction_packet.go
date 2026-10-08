package bidanalysisv3

import (
	"encoding/json"
	"fmt"

	"bid-engine/pkg/db/model"
)

func compactTableRows(dataJSON string) ([][]string, error) {
	var data struct {
		Grid [][]struct {
			Text string `json:"text"`
		} `json:"grid"`
	}
	if err := json.Unmarshal([]byte(dataJSON), &data); err != nil {
		return nil, err
	}
	if data.Grid == nil {
		return nil, fmt.Errorf("表格缺少 grid")
	}
	rows := make([][]string, len(data.Grid))
	for rowIndex, row := range data.Grid {
		rows[rowIndex] = make([]string, len(row))
		for columnIndex, cell := range row {
			rows[rowIndex][columnIndex] = cell.Text
		}
	}
	return rows, nil
}

func tableBelongsToChapter(table *model.BidAnalysisV3SourceTable, blocks []*model.BidAnalysisV3DocumentBlock) bool {
	if table == nil || len(blocks) == 0 {
		return false
	}
	first, last := blocks[0], blocks[len(blocks)-1]
	if table.PageStart < first.PageNo || table.PageStart > last.PageNo {
		return false
	}
	if table.PageStart == first.PageNo && table.SortOrder < first.SortOrder {
		return false
	}
	if table.PageStart == last.PageNo && table.SortOrder > last.SortOrder {
		return false
	}
	return true
}
