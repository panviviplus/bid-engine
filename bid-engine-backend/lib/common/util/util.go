package util

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"bid-engine/lib/common/cos"
)

// IsValidPhoneNum 是否是合法的手机号
func IsValidPhoneNum(phoneNum string) bool {
	return len(phoneNum) >= 11
}

// IsValidUserID 是否是合法的用户ID
func IsValidUserID(userID int64) bool {
	return userID > 0
}

// MakeFileMd5 获取文件的md5码
func MakeFileMd5(content []byte) string {
	h := md5.New()
	h.Write(content)
	return hex.EncodeToString(h.Sum(nil))
}

// GetFileMD5StringWithReader 获取文件 md5 字符串，reader: 文件句柄
func GetFileMD5StringWithReader(reader io.Reader) (string, error) {
	hash := md5.New()
	_, err := io.Copy(hash, reader)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// GetFilePathByFileIDBasePath 通过文件id构造文件路径
func GetFilePathByFileIDBasePath(FileID string, basePath string) string {
	uuidBytes := []byte(FileID)
	var u uuid.UUID
	decodeHex(u[:], uuidBytes)
	path := fmt.Sprintf("%s/%s/%s/%s",
		basePath,
		randomStr(binary.BigEndian.Uint32(u[0:4])),
		randomStr(binary.BigEndian.Uint32(u[4:8])),
		FileID)
	return path
}

// GenAvatarPath 生成头像cos存储路径
func GenAvatarPath(avatarID string, fileExt string) string {
	basePath := cos.BasePathAvatar
	filePath := GetFilePathByFileIDBasePath(avatarID, basePath)
	if fileExt != "" {
		if !strings.HasPrefix(fileExt, ".") {
			fileExt = "." + fileExt
		}
	}
	return filePath + "/0" + fileExt
}

func decodeHex(uuid []byte, src []byte) {
	_, _ = hex.Decode(uuid[0:4], src)
	_, _ = hex.Decode(uuid[4:6], src[9:13])
	_, _ = hex.Decode(uuid[6:8], src[14:18])
	_, _ = hex.Decode(uuid[8:10], src[19:23])
	_, _ = hex.Decode(uuid[10:], src[24:])
}

func randomStr(r uint32) string {
	r = r*1664525 + 1013904223 // constants from Numerical Recipes
	return strconv.Itoa(int(1000 + r%1000))[1:]
}

// HmacSha256 计算字符串 data 的 HmacSha256 编码， secret：秘钥
func HmacSha256(data string, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}
