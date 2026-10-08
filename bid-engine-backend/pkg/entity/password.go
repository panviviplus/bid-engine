package entity

import (
	"fmt"
	"regexp"
)

// passwordCharRegex 密码字符集校验：8-128位，仅允许数字、大小写字母和英文标点符号
var passwordCharRegex = regexp.MustCompile(`^[A-Za-z0-9!@#$%^&*()_+\-=\[\]{}|;':",./<>?]{8,128}$`)

// ValidatePassword 密码强度校验。
// 规则：8-128位，仅支持 ASCII 可打印字符中的数字、大小写字母和英文标点符号，
// 且必须包含大写字母、小写字母、数字、特殊符号中的至少3种。
func ValidatePassword(password string) error {
	if !passwordCharRegex.MatchString(password) {
		return fmt.Errorf("密码格式不正确：8-128位，仅支持数字、大小写字母和英文标点符号")
	}

	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, r := range password {
		switch {
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		case r >= 'a' && r <= 'z':
			hasLower = true
		case r >= '0' && r <= '9':
			hasDigit = true
		default:
			hasSpecial = true
		}
	}

	count := 0
	if hasUpper {
		count++
	}
	if hasLower {
		count++
	}
	if hasDigit {
		count++
	}
	if hasSpecial {
		count++
	}
	if count < 3 {
		return fmt.Errorf("密码强度不够：需包含大写字母、小写字母、数字、特殊符号中的至少3种")
	}

	return nil
}
