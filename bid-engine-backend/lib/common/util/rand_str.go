package util

import (
	"fmt"
	"math/rand"
	"time"
	"unsafe"
)

const letters = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

var src = rand.NewSource(time.Now().UnixNano())

const (
	// 6 bits to represent a letter index
	letterIDBits = 6
	// All 1-bits as many as letterIDBits
	letterIDMask = 1<<letterIDBits - 1
	letterIDMax  = 63 / letterIDBits
)

// RandStr 生成随机字符串
func RandStr(n int) string {
	b := make([]byte, n)
	// A rand.Int63() generates 63 random bits, enough for letterIDMax letters!
	for i, cache, remain := n-1, src.Int63(), letterIDMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIDMax
		}
		if idx := int(cache & letterIDMask); idx < len(letters) {
			b[i] = letters[idx]
			i--
		}
		cache >>= letterIDBits
		remain--
	}
	return *(*string)(unsafe.Pointer(&b))
}

// MakeSmsCode 生成短信验证码，验证码由6位数字构成
func MakeSmsCode() string {
	code := 100000 + rand.Intn(899999)
	return fmt.Sprint(code)
}
