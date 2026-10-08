package handler

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

func EqualSlicesSorted(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	// 排序切片
	sort.Strings(a)
	sort.Strings(b)
	// 逐个元素比较
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func GenerateUUID32() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// 工具函数：判断字符串是否在字符串切片中
func StringInSlice(str string, list []string) bool {
	if len(list) == 0 {
		return false
	}
	for _, item := range list {
		if item == str {
			return true
		}
	}
	return false
}

// 工具函数：判断int64是否在int64切片中
func Int64InSlice(i int64, list []int64) bool {
	if len(list) == 0 {
		return false
	}
	for _, item := range list {
		if item == i {
			return true
		}
	}
	return false
}

func FormatBidDeadline(bidDdlOri string) string {
	s := strings.TrimSpace(bidDdlOri)
	if s == "" {
		return bidDdlOri
	}
	ddl, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.Local)
	if err != nil {
		return bidDdlOri
	}
	now := time.Now()
	diff := ddl.Sub(now)
	sign := "还有"
	if diff < 0 {
		diff = -diff
		sign = "已经"
	}
	mins := int64(diff / time.Minute)
	days := mins / (24 * 60)
	hours := (mins % (24 * 60)) / 60
	minutes := mins % 60
	y := ddl.Year()
	m := int(ddl.Month())
	d := ddl.Day()
	hh := ddl.Hour()
	mm := ddl.Minute()
	if sign == "已经" {
		return fmt.Sprintf("%d年%d月%d日 %d时%d分，投标截止已经超过%d天%d时%d分", y, m, d, hh, mm, days, hours, minutes)
	}
	return fmt.Sprintf("%d年%d月%d日 %d时%d分，距离投标截止还有%d天%d时%d分", y, m, d, hh, mm, days, hours, minutes)
}

// IsAlphaCode 判断字符串是否全为字母（不限制长度）
func IsAlphaCode(v string) bool {
	s := strings.TrimSpace(v)
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		b := s[i]
		if !((b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')) {
			return false
		}
	}
	return true
}
