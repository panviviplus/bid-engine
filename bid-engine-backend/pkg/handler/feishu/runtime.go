package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"bid-engine/pkg/db/model"
	"github.com/larksuite/oapi-sdk-go/v3"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/larksuite/oapi-sdk-go/v3/ws"
)

// Start runs the official long-connection event client and the durable alert sender.
func (s *Service) Start(ctx context.Context) {
	if !s.Enabled() {
		s.logger.Warn("飞书接入未配置，跳过长连接和提醒投递")
		return
	}
	go s.runDelivery(ctx)
	handler := dispatcher.NewEventDispatcher("", "").OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
		if event == nil || event.EventV2Base == nil || event.EventV2Base.Header == nil || event.EventV2Base.Header.AppID != s.appID ||
			event.Event == nil || event.Event.Sender == nil || event.Event.Message == nil {
			return nil
		}
		sender, message := event.Event.Sender, event.Event.Message
		if sender.SenderType == nil || *sender.SenderType != "user" || sender.SenderId == nil || sender.SenderId.OpenId == nil ||
			message.ChatType == nil || *message.ChatType != "p2p" || message.MessageType == nil || *message.MessageType != "text" ||
			message.Content == nil || message.MessageId == nil {
			return nil
		}
		var content struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(*message.Content), &content); err != nil {
			return nil
		}
		parts := strings.Fields(strings.TrimSpace(content.Text))
		if len(parts) != 2 {
			return nil
		}
		purpose := ""
		switch parts[0] {
		case "登录":
			purpose = "login"
		case "绑定":
			purpose = "bind"
		default:
			return nil
		}
		tenantKey := ""
		if sender.TenantKey != nil {
			tenantKey = *sender.TenantKey
		}
		return s.confirmCode(ctx, purpose, parts[1], *sender.SenderId.OpenId, tenantKey, *message.MessageId)
	})
	client := ws.NewClient(s.appID, s.appSecret, ws.WithEventHandler(handler))
	if err := client.Start(ctx); err != nil {
		s.logger.Errorw("飞书事件长连接退出", "err", err)
	}
}

func (s *Service) runDelivery(ctx context.Context) {
	client := lark.NewClient(s.appID, s.appSecret)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		if err := s.deliverBatch(ctx, client); err != nil {
			s.logger.Errorw("飞书提醒投递失败", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Service) deliverBatch(ctx context.Context, client *lark.Client) error {
	alerts, err := s.repo.ClaimDueAlerts(ctx, time.Now(), 30)
	if err != nil || len(alerts) == 0 {
		return err
	}
	userIDs := make([]int64, 0, len(alerts))
	for _, alert := range alerts {
		userIDs = append(userIDs, alert.UserID)
	}
	recipients, err := s.repo.Recipients(ctx, s.appID, userIDs)
	if err != nil {
		return err
	}
	lastSend := make(map[int64]time.Time)
	for _, alert := range alerts {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		recipient, ok := recipients[alert.UserID]
		if !ok || recipient.Status != 1 || recipient.NotifyEnabled != 1 || recipient.OpenID == "" {
			if e := s.repo.FinishAlert(ctx, alert.ID, "skipped", "", "not_available", nil); e != nil {
				s.logger.Warnw("更新提醒跳过状态失败", "alert_id", alert.ID, "err", e)
			}
			continue
		}
		if previous := lastSend[alert.UserID]; !previous.IsZero() {
			wait := 210*time.Millisecond - time.Since(previous)
			if wait > 0 {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(wait):
				}
			}
		}
		lastSend[alert.UserID] = time.Now()
		messageID, code, sendErr := s.sendAlert(ctx, client, alert, recipient.OpenID)
		if sendErr == nil {
			if e := s.repo.FinishAlert(ctx, alert.ID, "sent", messageID, "", nil); e != nil {
				s.logger.Warnw("更新提醒发送状态失败", "alert_id", alert.ID, "err", e)
			}
			continue
		}
		status := "processing"
		var next *time.Time
		if permanentSendError(code) || alert.FeishuPushAttempts >= 4 {
			status = "failed"
		} else {
			backoff := 15 * time.Second * time.Duration(1<<alert.FeishuPushAttempts)
			at := time.Now().Add(backoff)
			next = &at
		}
		if e := s.repo.FinishAlert(ctx, alert.ID, status, "", code, next); e != nil {
			s.logger.Warnw("更新提醒重试状态失败", "alert_id", alert.ID, "err", e)
		}
		s.logger.Warnw("飞书提醒发送失败", "alert_id", alert.ID, "code", code, "err", sendErr)
	}
	return nil
}

func permanentSendError(code string) bool {
	switch code {
	case "230013", "230029", "230034", "230035", "230053":
		return true
	}
	return false
}

func (s *Service) sendAlert(ctx context.Context, client *lark.Client, alert model.TenderIntelAlert, openID string) (string, string, error) {
	content := fmt.Sprintf("标擎 · 招标情报提醒\n订阅：%s\n公告：%s", cleanText(alert.SubscriptionName, 80), cleanText(alert.NoticeTitle, 180))
	if reason := cleanText(alert.MatchedReason, 180); reason != "" {
		content += "\n命中原因：" + reason
	}
	if s.publicBaseURL != "" {
		content += fmt.Sprintf("\n查看详情：%s/intel/%d?subscription=%d", s.publicBaseURL, alert.NoticeID, alert.SubscriptionID)
	}
	body, err := json.Marshal(map[string]string{"text": content})
	if err != nil {
		return "", "encoding", err
	}
	req := larkim.NewCreateMessageReqBuilder().ReceiveIdType("open_id").Body(
		larkim.NewCreateMessageReqBodyBuilder().ReceiveId(openID).MsgType("text").Content(string(body)).Uuid(fmt.Sprintf("feishu-intel-%d", alert.ID)).Build(),
	).Build()
	resp, err := client.Im.V1.Message.Create(ctx, req)
	if err != nil {
		return "", "transport", err
	}
	if !resp.Success() {
		return "", fmt.Sprint(resp.Code), fmt.Errorf("feishu API: %s", resp.Msg)
	}
	if resp.Data != nil && resp.Data.MessageId != nil {
		return *resp.Data.MessageId, "", nil
	}
	return "", "empty_response", fmt.Errorf("feishu API returned no message id")
}

func cleanText(value string, max int) string {
	value = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(value, "\r", " "), "\n", " "))
	runes := []rune(value)
	if len(runes) > max {
		return string(runes[:max]) + "…"
	}
	return value
}
