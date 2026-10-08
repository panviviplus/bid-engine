package encrypt

import (
	"bytes"
	"crypto/cipher"
	"encoding/base64"
	"fmt"
	"os"
	"bid-engine/lib/common/logtool"
	"strings"

	"github.com/tjfoc/gmsm/sm4"
)

var (
	sm4Key   = ""
	iv       = ""
	sm4Ready = false
)

const (
	keyPre = "BEE_ENC_COMMON_"
)

func init() {
	sm4Key = os.Getenv("SM4_KEY")
	iv = os.Getenv("IV_KEY")
	if sm4Key == "" || iv == "" {
		// 本地测试环境允许不配置密钥，降级为透传，避免影响编译/单测。
		if os.Getenv("DEBUG") == "true" {
			logtool.GetLogger().Sugar().Warnw("SM4未配置，启用透传模式")
		}
		return
	}
	sm4Ready = true
}

func pkcs7Padding(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padtext...)
}

func pkcs7Unpadding(data []byte) []byte {
	length := len(data)
	if length == 0 {
		return nil
	}
	unpadding := int(data[length-1])
	return data[:(length - unpadding)]
}

func SM4EncryptBase64(data []byte) (string, error) {
	if !sm4Ready {
		return string(data), nil
	}
	block, err := sm4.NewCipher([]byte(sm4Key))
	if err != nil {
		return "", err
	}

	data = pkcs7Padding(data, block.BlockSize())
	ciphertext := make([]byte, len(data))

	mode := cipher.NewCBCEncrypter(block, []byte(iv))
	mode.CryptBlocks(ciphertext, data)

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func SM4DecryptBase64(b64 string) (string, error) {
	if !sm4Ready {
		return b64, nil
	}
	block, err := sm4.NewCipher([]byte(sm4Key))
	if err != nil {
		return b64, err
	}
	if strings.HasPrefix(b64, keyPre) {
		b64 = b64[len(keyPre):]
	}
	ciphertext, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return b64, err
	}
	if len(ciphertext) == 0 || len(ciphertext)%sm4.BlockSize != 0 {
		return b64, fmt.Errorf("invalid ciphertext length")
	}

	plaintext := make([]byte, len(ciphertext))
	mode := cipher.NewCBCDecrypter(block, []byte(iv))
	mode.CryptBlocks(plaintext, ciphertext)

	ret, err := safePKCS7Unpadding(plaintext)

	if os.Getenv("DEBUG") == "true" {
		logtool.GetLogger().Sugar().Infow("SM4DecryptBase64 结果", "b64", b64, "ret", string(ret))
	}
	return string(ret), err
}

func safePKCS7Unpadding(data []byte) ([]byte, error) {
	length := len(data)
	if length == 0 {
		return nil, fmt.Errorf("invalid padding: empty data")
	}

	padding := int(data[length-1])

	// padding 不合法
	if padding == 0 || padding > sm4.BlockSize || padding > length {
		return nil, fmt.Errorf("invalid padding: %d", padding)
	}

	// 每个 padding 位都必须相同
	pad := data[length-padding:]
	for _, v := range pad {
		if int(v) != padding {
			return nil, fmt.Errorf("invalid padding bytes")
		}
	}

	return data[:length-padding], nil
}
