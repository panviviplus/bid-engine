package bidgen

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	goredis "github.com/redis/go-redis/v9"

	skbcfg "bid-engine/pkg/config"
	"bid-engine/pkg/db/model"
	"bid-engine/pkg/entity"
)

const (
	generationEventPrefix   = "bidgen:events:"
	generationPartialPrefix = "bidgen:partial:"
)

type generationEvent struct {
	Event string
	Data  string
}

type generationSnapshot struct {
	TaskID           int64  `json:"taskId"`
	Status           string `json:"status"`
	Progress         int32  `json:"progress"`
	Current          int32  `json:"current"`
	Total            int32  `json:"total"`
	CurrentOutlineID int64  `json:"currentOutlineId"`
	Title            string `json:"title,omitempty"`
	PartialText      string `json:"partialText,omitempty"`
	EventID          string `json:"-"`
}

func bidGenConfigInt(key string, fallback, minimum, maximum int) int {
	value := fallback
	if raw := strings.TrimSpace(skbcfg.Get(key)); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			value = parsed
		}
	}
	if value < minimum {
		return minimum
	}
	if maximum > 0 && value > maximum {
		return maximum
	}
	return value
}

func generationEventKey(taskID int64) string {
	return fmt.Sprintf("%s%d", generationEventPrefix, taskID)
}

func generationPartialKey(taskID int64) string {
	return fmt.Sprintf("%s%d", generationPartialPrefix, taskID)
}

func (s *svcImpl) publishGenerationEvent(ctx context.Context, taskID int64, event string, payload any) (string, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	maxLen := int64(bidGenConfigInt("bid_generation.event_stream_max_len", 4096, 256, 20000))
	ttl := time.Duration(bidGenConfigInt("bid_generation.event_ttl_sec", 86400, 3600, 604800)) * time.Second
	key := generationEventKey(taskID)
	id, err := s.redisSvc.Client().XAdd(ctx, &goredis.XAddArgs{
		Stream: key,
		MaxLen: maxLen,
		Approx: true,
		Values: map[string]any{"event": event, "data": string(raw)},
	}).Result()
	if err != nil {
		return "", err
	}
	_ = s.redisSvc.Client().Expire(ctx, key, ttl).Err()
	return id, nil
}

func (s *svcImpl) publishGenerationDelta(ctx context.Context, taskID, outlineID int64, title, delta, accumulated string) error {
	raw, err := json.Marshal(sseDelta{OutlineID: outlineID, Text: delta})
	if err != nil {
		return err
	}
	maxLen := bidGenConfigInt("bid_generation.event_stream_max_len", 4096, 256, 20000)
	ttl := time.Duration(bidGenConfigInt("bid_generation.event_ttl_sec", 86400, 3600, 604800)) * time.Second
	_, err = publishDeltaScript.Run(ctx, s.redisSvc.Client(), []string{
		generationEventKey(taskID), generationPartialKey(taskID),
	}, maxLen, int(ttl.Seconds()), string(raw), outlineID, title, accumulated).Result()
	return err
}

var publishDeltaScript = goredis.NewScript(`
local id = redis.call("XADD", KEYS[1], "MAXLEN", "~", ARGV[1], "*", "event", "delta", "data", ARGV[3])
redis.call("HSET", KEYS[2], "outline_id", ARGV[4], "title", ARGV[5], "text", ARGV[6], "event_id", id)
redis.call("EXPIRE", KEYS[1], ARGV[2])
redis.call("EXPIRE", KEYS[2], ARGV[2])
return id
`)

func (s *svcImpl) clearGenerationPartial(ctx context.Context, taskID int64) {
	_ = s.redisSvc.Client().Del(ctx, generationPartialKey(taskID)).Err()
}

func (s *svcImpl) loadGenerationSnapshot(ctx context.Context, task *model.BidGenTask) generationSnapshot {
	snapshot := generationSnapshot{
		TaskID: task.ID, Status: task.Status, Progress: task.Progress,
		Current: task.CompletedCount, Total: task.TotalCount, CurrentOutlineID: task.CurrentOutlineID,
	}
	values, err := s.redisSvc.Client().HGetAll(ctx, generationPartialKey(task.ID)).Result()
	if err != nil || len(values) == 0 {
		return snapshot
	}
	snapshot.Title = values["title"]
	snapshot.PartialText = values["text"]
	snapshot.EventID = values["event_id"]
	if id, err := strconv.ParseInt(values["outline_id"], 10, 64); err == nil && id > 0 {
		snapshot.CurrentOutlineID = id
	}
	return snapshot
}

// SubscribeGenerateEvents 将持久任务事件流式发送给当前页面。请求断开只结束订阅，不影响 Worker。
func (s *svcImpl) SubscribeGenerateEvents(c *gin.Context) {
	ctx := c.Request.Context()
	projectID := parseInt64(c.Query("projectId"))
	taskID := parseInt64(c.Query("taskId"))
	if projectID <= 0 || taskID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "projectId 和 taskId 不能为空"})
		return
	}
	task, err := s.repo.GetTaskForUser(ctx, entity.GetUserIDFromCtx(c), taskID)
	if err != nil || task == nil || task.ProjectID != projectID {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "生成任务不存在"})
		return
	}

	setSSEHeaders(c)
	_, _ = c.Writer.WriteString("retry: 2000\n\n")
	c.Writer.Flush()

	lastID := strings.TrimSpace(c.GetHeader("Last-Event-ID"))
	if lastID == "" {
		if latest, _ := s.redisSvc.Client().XRevRangeN(ctx, generationEventKey(taskID), "+", "-", 1).Result(); len(latest) > 0 {
			lastID = latest[0].ID
		}
	}
	if fresh, freshErr := s.repo.GetTaskByID(ctx, taskID); freshErr == nil && fresh != nil {
		task = fresh
	}
	snapshot := s.loadGenerationSnapshot(ctx, task)
	sendSSEEvent(c, "task_state", snapshot)
	if isGenerationTerminal(task.Status) {
		s.sendRecoveredTerminal(c, task)
		return
	}
	sendSSEEvent(c, "snapshot", snapshot)

	if strings.TrimSpace(c.GetHeader("Last-Event-ID")) == "" && snapshot.EventID != "" {
		lastID = snapshot.EventID
	}
	if lastID == "" {
		lastID = "0-0"
	}

	heartbeat := time.Duration(bidGenConfigInt("bid_generation.sse_heartbeat_sec", 15, 5, 60)) * time.Second
	for {
		streams, readErr := s.redisSvc.Client().XRead(ctx, &goredis.XReadArgs{
			Streams: []string{generationEventKey(taskID), lastID}, Count: 100, Block: heartbeat,
		}).Result()
		if readErr != nil && readErr != goredis.Nil {
			return
		}
		if len(streams) == 0 {
			_, _ = c.Writer.WriteString(": ping\n\n")
			c.Writer.Flush()
			fresh, freshErr := s.repo.GetTaskByID(ctx, taskID)
			if freshErr == nil && isGenerationTerminal(fresh.Status) {
				s.sendRecoveredTerminal(c, fresh)
				return
			}
			continue
		}
		for _, stream := range streams {
			for _, message := range stream.Messages {
				event, _ := message.Values["event"].(string)
				data, _ := message.Values["data"].(string)
				if event == "" || data == "" {
					lastID = message.ID
					continue
				}
				sendRawSSEEvent(c, message.ID, event, data)
				lastID = message.ID
				if isTerminalGenerationEvent(event) {
					return
				}
			}
		}
	}
}

func (s *svcImpl) sendRecoveredTerminal(c *gin.Context, task *model.BidGenTask) {
	switch task.Status {
	case "succeeded":
		sendSSEEvent(c, "done", gin.H{"taskId": task.ID})
	case "cancelled":
		sendSSEEvent(c, "cancelled", gin.H{"taskId": task.ID})
	default:
		sendSSEEvent(c, "error", sseError{Msg: task.ErrorMsg})
	}
}

func sendRawSSEEvent(c *gin.Context, id, event, data string) {
	if id != "" {
		_, _ = c.Writer.WriteString("id: " + id + "\n")
	}
	_, _ = c.Writer.WriteString("event: " + event + "\n")
	_, _ = c.Writer.WriteString("data: " + data + "\n\n")
	c.Writer.Flush()
}

func isGenerationTerminal(status string) bool {
	return status == "succeeded" || status == "failed" || status == "cancelled"
}

func isTerminalGenerationEvent(event string) bool {
	return event == "done" || event == "error" || event == "cancelled"
}
