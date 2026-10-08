package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"time"

	skbcfg "bid-engine/pkg/config"
)

const (
	ZhidaUploadFileDomain = "http://localhost:1022" // placeholder for local dev
	ZhidaUploadFileUrl    = "/v1/files/upload"
	ZhidaDefaultUser      = "abc-123"
)

type difyUploadFileResp struct {
	ID string `json:"id"`
}

// UploadFileToDify 上传文件给智搭的目标应用，单个上传
// @param apiKey: 智搭应用的API密钥
// @param localFilePath: 本地文件路径
// @return 文件ID和错误信息
func UploadFileToDify(apiKey, localFilePath string) (string, error) {
	reqDomain := skbcfg.Get("properties.zhida_req_domain")
	if strings.TrimSpace(reqDomain) == "" {
		reqDomain = ZhidaUploadFileDomain
	}
	reqUrl := reqDomain + ZhidaUploadFileUrl
	return uploadFileToDify(apiKey, localFilePath, reqUrl, ZhidaDefaultUser)
}

func uploadFileToDify(apiKey, localFilePath, uploadURL, user string) (string, error) {
	if strings.TrimSpace(apiKey) == "" {
		return "", fmt.Errorf("apiKey is empty")
	}
	if strings.TrimSpace(localFilePath) == "" {
		return "", fmt.Errorf("localFilePath is empty")
	}

	file, err := os.Open(localFilePath)
	if err != nil {
		return "", fmt.Errorf("open file failed: %w", err)
	}
	defer file.Close()

	mimeType, err := detectMimeType(file, localFilePath)
	if err != nil {
		return "", err
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	if writeErr := writer.WriteField("user", user); writeErr != nil {
		return "", fmt.Errorf("write field user failed: %w", writeErr)
	}

	fileName := filepath.Base(localFilePath)
	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, "file", escapeQuotes(fileName)))
	partHeader.Set("Content-Type", mimeType)
	part, err := writer.CreatePart(partHeader)
	if err != nil {
		return "", fmt.Errorf("create file part failed: %w", err)
	}

	if _, copyErr := io.Copy(part, file); copyErr != nil {
		return "", fmt.Errorf("write file part failed: %w", copyErr)
	}

	if closeErr := writer.Close(); closeErr != nil {
		return "", fmt.Errorf("close multipart writer failed: %w", closeErr)
	}

	req, err := http.NewRequest(http.MethodPost, uploadURL, body)
	if err != nil {
		return "", fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("%s", apiKey))
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 90 * time.Second}

	log.Printf("Dify文件上传请求: method=%s url=%s headers={Authorization=%s, Content-Type=%s} form={user=%s, file_name=%s, file_type=%s, content_length=%d}",
		http.MethodPost,
		uploadURL,
		fmt.Sprintf("%s", apiKey),
		writer.FormDataContentType(),
		user,
		fileName,
		mimeType,
		body.Len(),
	)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response failed: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("智搭上传文件失败: status=%d body=%s", resp.StatusCode, truncateBytes(respBody, 2048))
	}

	var parsed difyUploadFileResp
	if unmarshalErr := json.Unmarshal(respBody, &parsed); unmarshalErr != nil {
		return "", fmt.Errorf("解析响应值失败: %w", unmarshalErr)
	}
	if strings.TrimSpace(parsed.ID) == "" {
		return "", fmt.Errorf("智搭上传文件返回值里没有文件ID")
	}

	return parsed.ID, nil
}

func detectMimeType(file *os.File, localFilePath string) (string, error) {
	ext := strings.ToLower(filepath.Ext(localFilePath))
	if ext != "" {
		if t := mime.TypeByExtension(ext); t != "" {
			return t, nil
		}
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("seek file failed: %w", err)
	}

	buf := make([]byte, 512)
	n, _ := file.Read(buf)
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("seek file failed: %w", err)
	}

	if n == 0 {
		return "application/octet-stream", nil
	}

	return http.DetectContentType(buf[:n]), nil
}

func escapeQuotes(s string) string {
	return strings.ReplaceAll(s, `"`, `\"`)
}

func truncateBytes(b []byte, max int) string {
	if max <= 0 {
		return ""
	}
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "...(truncated)"
}
