package util

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"sort"
)

// SignParam 生成签名需要的参数
type SignParam struct {
	Body    string // body数据
	Query   string // 原始query
	DateGMT string // GTM时间
	Nonce   string // 随机数
}

func getMD5(str string) []byte {
	h := md5.New()
	h.Write([]byte(str))
	return h.Sum(nil)
}

func hmacSha256(data string, secret string) []byte {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(data))
	return h.Sum(nil)
}

func resortQuery(src string) string {
	queries, _ := url.ParseQuery(src)
	keys := make([]string, 0)
	for k := range queries {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	newQuery := url.Values{}
	for _, k := range keys {
		for _, value := range queries[k] {
			newQuery.Add(k, value)
		}
	}
	return newQuery.Encode()
}

// GenSignature 生成签名
func GenSignature(src SignParam, apiSecret string) (string, string) {
	// 计算body的md5值
	md5str := getMD5(src.Body)
	// base64后得到contentMD5
	contentMD5 := base64.StdEncoding.EncodeToString(md5str)
	// query解析，并按照字典序重新排列
	query := resortQuery(src.Query)
	query, _ = url.QueryUnescape(query)
	// 需要做签名的字符串结构
	stringToSign := `POST
application/json
%s
application/json
%s
HMAC-SHA256
%s
%s`
	stringToSign = fmt.Sprintf(stringToSign, contentMD5, src.DateGMT, src.Nonce, query)
	hmac256 := hmacSha256(stringToSign, apiSecret)
	signature := base64.StdEncoding.EncodeToString(hmac256)
	return contentMD5, signature
}
