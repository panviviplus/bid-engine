package utils

import (
	"strconv"
	"strings"
	"time"
)

func ParseDateToUnix(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	digits := true
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			digits = false
			break
		}
	}
	if digits {
		v, _ := strconv.ParseInt(s, 10, 64)
		if v > 1000000000000 {
			v = v / 1000
		}
		return v
	}
	layouts := []string{
		"2006-01-02",
		"2006/01/02",
		"2006.01.02",
		"2006-01-02 15:04:05",
		"2006/01/02 15:04:05",
		"2006.01.02 15:04:05",
		"2006年01月02日",
		"2006年1月2日",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t.Unix()
		}
	}
	return 0
}
