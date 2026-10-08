package utils

import (
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

func ReadExcelAsList(localPath string) ([]map[string]string, error) {
	f, err := excelize.OpenFile(localPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sheets := f.GetSheetList()
	sheet := "Sheet1"
	if len(sheets) > 0 {
		sheet = sheets[0]
	}
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 {
		return []map[string]string{}, nil
	}
	header := rows[0]
	result := make([]map[string]string, 0, len(rows)-1)
	for i := 1; i < len(rows); i++ {
		line := rows[i]
		item := make(map[string]string, len(header))
		for colIdx, colName := range header {
			name := strings.TrimSpace(colName)
			if name == "" {
				continue
			}
			val := ""
			if colIdx < len(line) {
				val = strings.TrimSpace(line[colIdx])
			}
			item[name] = val
		}
		result = append(result, item)
	}
	return result, nil
}

func ReadOriginExcelAsList(localPath string) ([]map[string]string, error) {
	f, err := excelize.OpenFile(localPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sheets := f.GetSheetList()
	sheet := "Sheet1"
	if len(sheets) > 0 {
		sheet = sheets[0]
	}
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 {
		return []map[string]string{}, nil
	}
	header := rows[0]
	result := make([]map[string]string, 0, len(rows)-1)
	for r := 2; r <= len(rows); r++ {
		item := make(map[string]string, len(header))
		for c := 1; c <= len(header); c++ {
			name := strings.TrimSpace(header[c-1])
			if name == "" {
				continue
			}
			axis, _ := excelize.CoordinatesToCellName(c, r)
			disp, _ := f.GetCellValue(sheet, axis)
			item[name] = strings.TrimSpace(normalizeDisplayDate(disp))
		}
		result = append(result, item)
	}
	return result, nil
}

func normalizeDisplayDate(s string) string {
	v := strings.TrimSpace(s)
	if v == "" {
		return v
	}
	layoutsDT := []string{
		"2006-01-02 15:04:05",
		"2006/01/02 15:04:05",
		"2006-01-02 15:04",
		"2006/01/02 15:04",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
	}
	for _, l := range layoutsDT {
		if t, err := time.Parse(l, v); err == nil {
			return t.Format("2006-01-02 15:04:05")
		}
	}
	layoutsDate := []string{
		"2006-01-02",
		"2006/01/02",
		"01-02-06",
		"01/02/06",
		"02-01-06",
		"02/01/06",
		"2006.01.02",
	}
	for _, l := range layoutsDate {
		if t, err := time.Parse(l, v); err == nil {
			return t.Format("2006-01-02")
		}
	}
	return v
}
