package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	skbcfg "bid-engine/pkg/config"
)

type emailReq struct {
	EmailContent  string `json:"emailContent"`
	EmailType     int    `json:"emailType"`
	SystemCode    string `json:"systemCode"`
	SystemName    string `json:"systemName"`
	TemplateCode  string `json:"templateCode"`
	SendEmail     string `json:"sendEmail"`
	AcceptorEmail string `json:"acceptorEmail"`
	EmailBcc      string `json:"emailBcc"`
	EmailCc       string `json:"emailCc"`
	EmailParam    string `json:"emailParam"`
	EmailTitle    string `json:"emailTitle"`
}

func sendTemplateEmail(ctx context.Context, body emailReq, label string) error {
	url := strings.TrimSpace(skbcfg.Get("bidhub.send_email.url"))
	keyName := fmt.Sprintf("bidhub.send_email.%s", label)
	key := strings.TrimSpace(skbcfg.Get(keyName))
	to := strings.TrimSpace(body.AcceptorEmail)

	if url == "" {
		url = "http://localhost:1022/api/mock/email/send"
	}
	if key == "" {
		key = "61623e69-d3cf-4f0a-91a5-ca8a3263e9df"
	}
	if to == "" {
		return fmt.Errorf("acceptor email empty")
	}
	timeout := 5
	if v := strings.TrimSpace(skbcfg.Get("bidhub.send_email.timeout")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			timeout = n
		}
	}
	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("keyid", key)
	cli := &http.Client{Timeout: time.Duration(timeout) * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("email api status %d", resp.StatusCode)
	}
	return nil
}

func SendReviewProjectEmail(ctx context.Context, acceptorEmail, projectName, result, cc string) error {
	body := emailReq{
		EmailContent:  "",
		EmailType:     1,
		SystemCode:    "01132-aiSolution",
		SystemName:    "商业投标助手",
		TemplateCode:  "winbid_review",
		SendEmail:     "aisolution@bid-engine.local",
		AcceptorEmail: strings.TrimSpace(acceptorEmail),
		EmailBcc:      "",
		EmailCc:       strings.TrimSpace(cc),
		EmailParam:    fmt.Sprintf("%s,%s", strings.TrimSpace(projectName), strings.TrimSpace(result)),
		EmailTitle:    "商业投标助手-投标文件审核完成通知",
	}
	return sendTemplateEmail(ctx, body, "review_key")
}

func SendQualificationExpireEmail(ctx context.Context, acceptorEmail, qualName, expireDate, cc string) error {
	body := emailReq{
		EmailContent:  "",
		EmailType:     1,
		SystemCode:    "01132-aiSolution",
		SystemName:    "商业投标助手",
		TemplateCode:  "winbid_overdue",
		SendEmail:     "aisolution@bid-engine.local",
		AcceptorEmail: strings.TrimSpace(acceptorEmail),
		EmailBcc:      "",
		EmailCc:       strings.TrimSpace(cc),
		EmailParam:    fmt.Sprintf("%s,%s", strings.TrimSpace(qualName), strings.TrimSpace(expireDate)),
		EmailTitle:    "商业投标助手-资证超期通知",
	}
	return sendTemplateEmail(ctx, body, "expire_key")
}
