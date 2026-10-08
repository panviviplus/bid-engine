package entity

import "strings"

// IsDuplicateErr 是否是重复错误
func IsDuplicateErr(err error) bool {
	return strings.Contains(err.Error(), "Duplicate")
}
