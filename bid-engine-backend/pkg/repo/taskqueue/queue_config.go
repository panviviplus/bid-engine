package taskqueue

import (
	"fmt"
	"strings"
	"time"

	skbcfg "bid-engine/pkg/config"
)

// QueueConfig 单个任务队列的运行时配置
type QueueConfig struct {
	Name          string        // 队列中文名（如 "招标解析"），也用作 task_type
	QueueKey      string        // Redis List key，如 task:queue:tender_parse
	HighQueueKey  string        // 高优先级队列 key
	DLQKey        string        // 死信队列 key
	Concurrency   int           // 消费并发数（同时处理几个任务）
	MaxAttempts   int           // 最大重试次数
	VisibilitySec int           // 租约超时（秒），超时未完成视为僵尸，交还队列
	RetryBackoff  time.Duration // 重试退避基础间隔
	PollInterval  time.Duration // BRPOP 长轮询阻塞时间
}

// DefaultQueueConfigs 兜底默认配置（conf-local.yml 未配置时生效）
var DefaultQueueConfigs = map[string]QueueConfig{
	"tender_parse_v3": {
		Name: "招标解析V3", QueueKey: "task:queue:tender_parse_v3", HighQueueKey: "task:queue:tender_parse_v3:high",
		// 页块与 LLM 调用在流水线内部各自重试；顶层失败后停在原阶段，交由用户选择恢复策略。
		DLQKey: "task:dlq:tender_parse_v3", Concurrency: 2, MaxAttempts: 1,
		VisibilitySec: 10800, RetryBackoff: 30 * time.Second, PollInterval: 5 * time.Second,
	},
	"tender_blueprint_v3": {
		Name: "标书蓝图V3", QueueKey: "task:queue:tender_blueprint_v3", HighQueueKey: "task:queue:tender_blueprint_v3:high",
		// 蓝图内部完成 JSON 修复；LLM 基础设施类瞬时故障（不可达/限流/超时）自动退避重试，
		// 内容校验类失败由 handler 标记为不可重试直接终态；最终失败保留失败态，由用户主动重试。
		DLQKey: "task:dlq:tender_blueprint_v3", Concurrency: 2, MaxAttempts: 3,
		VisibilitySec: 3600, RetryBackoff: 30 * time.Second, PollInterval: 5 * time.Second,
	},
	"material_parse": {
		Name: "素材库解析", QueueKey: "task:queue:material_parse", HighQueueKey: "task:queue:material_parse:high",
		DLQKey: "task:dlq:material_parse", Concurrency: 3, MaxAttempts: 3,
		VisibilitySec: 1800, RetryBackoff: 30 * time.Second, PollInterval: 5 * time.Second,
	},
	"bid_gen_parse": {
		Name: "标书生成解析", QueueKey: "task:queue:bid_gen_parse", HighQueueKey: "task:queue:bid_gen_parse:high",
		DLQKey: "task:dlq:bid_gen_parse", Concurrency: 2, MaxAttempts: 3,
		VisibilitySec: 7200, RetryBackoff: 30 * time.Second, PollInterval: 5 * time.Second,
	},
	"bid_gen_generate": {
		Name: "标书正文生成", QueueKey: "task:queue:bid_gen_generate", HighQueueKey: "task:queue:bid_gen_generate:high",
		DLQKey: "task:dlq:bid_gen_generate", Concurrency: 20, MaxAttempts: 3,
		VisibilitySec: 21600, RetryBackoff: 15 * time.Second, PollInterval: time.Second,
	},
	"bid_review": {
		Name: "投标书审核", QueueKey: "task:queue:bid_review", HighQueueKey: "task:queue:bid_review:high",
		DLQKey: "task:dlq:bid_review", Concurrency: 2, MaxAttempts: 3,
		VisibilitySec: 10800, RetryBackoff: 30 * time.Second, PollInterval: 5 * time.Second,
	},
	// 招标情报站：采集（网络型，并发抓取多个源）与打标（LLM 型，批量调用全局模型）
	// 拆成两个队列，避免网络等待与 LLM 等待互相抢占并发额度。
	"tender_intel_collect": {
		Name: "招标情报采集", QueueKey: "task:queue:tender_intel_collect", HighQueueKey: "task:queue:tender_intel_collect:high",
		DLQKey: "task:dlq:tender_intel_collect", Concurrency: 3, MaxAttempts: 2,
		VisibilitySec: 900, RetryBackoff: 30 * time.Second, PollInterval: 5 * time.Second,
	},
	"tender_intel_enrich": {
		Name: "招标情报打标", QueueKey: "task:queue:tender_intel_enrich", HighQueueKey: "task:queue:tender_intel_enrich:high",
		DLQKey: "task:dlq:tender_intel_enrich", Concurrency: 2, MaxAttempts: 2,
		VisibilitySec: 3600, RetryBackoff: 30 * time.Second, PollInterval: 5 * time.Second,
	},
	// 招标情报订阅匹配：纯内存比对 + 批量写提醒，与采集/打标彻底解耦，
	// 避免“用户多、订阅多”时拖住信息入库与打标。
	"tender_intel_match": {
		Name: "招标情报订阅匹配", QueueKey: "task:queue:tender_intel_match", HighQueueKey: "task:queue:tender_intel_match:high",
		DLQKey: "task:dlq:tender_intel_match", Concurrency: 2, MaxAttempts: 3,
		VisibilitySec: 1800, RetryBackoff: 30 * time.Second, PollInterval: 5 * time.Second,
	},
}

// 全局延迟队列 key（所有模块共享一个 Sorted Set）
const DelayedQueueKey = "task:delayed"

// LoadQueueConfigs 从 conf-local.yml 的 task_queues 块加载队列配置
// 未配置的字段回退到 DefaultQueueConfigs
func LoadQueueConfigs() map[string]QueueConfig {
	result := make(map[string]QueueConfig, len(DefaultQueueConfigs))
	for taskType, def := range DefaultQueueConfigs {
		cfg := def // 复制一份
		prefix := fmt.Sprintf("task_queues.%s", taskType)

		if v := strings.TrimSpace(skbcfg.Get(prefix + ".name")); v != "" {
			cfg.Name = v
		}
		if v := skbcfg.Get(prefix + ".concurrency"); v != "" {
			fmt.Sscanf(v, "%d", &cfg.Concurrency)
		}
		if v := skbcfg.Get(prefix + ".max_attempts"); v != "" {
			fmt.Sscanf(v, "%d", &cfg.MaxAttempts)
		}
		if v := skbcfg.Get(prefix + ".visibility_sec"); v != "" {
			fmt.Sscanf(v, "%d", &cfg.VisibilitySec)
		}
		if v := skbcfg.Get(prefix + ".retry_backoff_sec"); v != "" {
			var sec int
			fmt.Sscanf(v, "%d", &sec)
			cfg.RetryBackoff = time.Duration(sec) * time.Second
		}
		if v := skbcfg.Get(prefix + ".poll_interval_sec"); v != "" {
			var sec int
			fmt.Sscanf(v, "%d", &sec)
			cfg.PollInterval = time.Duration(sec) * time.Second
		}

		result[taskType] = cfg
	}
	return result
}

// QueueConfigs 已加载的队列配置（init 时填充）
var QueueConfigs map[string]QueueConfig
