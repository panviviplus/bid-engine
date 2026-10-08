package agent

import (
	"context"
	"strings"
	"sync"

	skbcfg "bid-engine/pkg/config"
	"bid-engine/lib/common/logtool"
)

var (
	instance Service
	once     sync.Once
)

// Service agent服务接口定义
type Service interface {
	// CallAgent 调用指定的agent服务（一次性返回）
	CallAgent(ctx context.Context, agentID string, req *AgentRequest) (*AgentResponse, error)

	// CallAgentStream 调用指定的agent服务（流式返回）
	CallAgentStream(ctx context.Context, agentID string, req *AgentRequest) (<-chan *AgentResponse, <-chan error)

	// GetAgentConfig 获取agent配置信息
	GetAgentConfig(agentID string) (*AgentConfig, error)

	// IsValidAgent 验证agent ID是否有效
	IsValidAgent(agentID string) bool
}

// AgentConfig agent配置信息
type AgentConfig struct {
	Id          string `json:"id" comment:"agent标识"`
	Name        string `json:"name" comment:"agent名称"`
	URL         string `json:"url" comment:"API地址"`
	Token       string `json:"token" comment:"认证Api Key"`
	Timeout     int    `json:"timeout" comment:"超时时间(秒)"`
	Description string `json:"description" comment:"描述"`
}

// AgentRequest agent请求结构
type AgentRequest struct {
	ResponseMode   string                 `json:"response_mode" comment:"响应模式，blocking、streaming"`
	ConversationID string                 `json:"conversation_id" comment:"会话ID"`
	Query          string                 `json:"query" comment:"用户问题"`
	Inputs         map[string]interface{} `json:"inputs" comment:"输入参数"`
	User           string                 `json:"user" comment:"用户标识"`
	Files          []interface{}          `json:"files" comment:"文件列表"`
}

// AgentResponse agent响应结构
type AgentResponse struct {
	Event          string                 `json:"event" comment:"事件类型"`
	TaskID         string                 `json:"task_id" comment:"任务ID"`
	ID             string                 `json:"id" comment:"消息ID"`
	MessageID      string                 `json:"message_id" comment:"消息ID"`
	ConversationID string                 `json:"conversation_id" comment:"会话ID"`
	Mode           string                 `json:"mode" comment:"响应模式"`
	Answer         string                 `json:"answer" comment:"AI回答内容（可能为JSON字符串）"`
	Metadata       map[string]interface{} `json:"metadata" comment:"附加元数据"`
	CreatedAt      int64                  `json:"created_at" comment:"创建时间戳(秒)"`
	AnswerParsed   map[string]string      `json:"-" comment:"AI回答解析后的键值对（从JSON字符串反序列化）"`
}

// GetInstance 获取agent服务实例
func GetInstance() Service {
	once.Do(func() {
		logger := logtool.GetLogger().Sugar()
		instance = &svcImpl{
			logger:       logger,
			agentConfigs: initAgentConfigs(),
		}
	})
	return instance
}

// initAgentConfigs 根据环境，初始化对应的agent配置
func initAgentConfigs() map[string]*AgentConfig {
	//agentEnv := "dev"
	//curEnv := skbcfg.Get("env")
	//if curEnv != "" {
	//	agentEnv = curEnv
	//}
	//if agentEnv == "zwy" {
	//	return getZwyAgentConfigs()
	//}
	return getAgentConfigs()
}

// getAgentConfigs 从配置文件读取智能体配置
func getAgentConfigs() map[string]*AgentConfig {
	agentURL := skbcfg.Get("agent.req_url")
	if strings.TrimSpace(agentURL) == "" {
		agentURL = "http://localhost:1022/v1/chat-messages" // placeholder for local dev
	}
	//env := strings.TrimSpace(skbcfg.Get("env"))
	def := map[string]string{
		"tender":             "app-mA64VP00oNPzzUI1dp1ddVNf",
		"tender-trace-index": "app-JAH7Fr03e2n4msO09n4eLqmt",
		"bid-format":         "app-XXw93lY2iXslKpUdGUMyaaa9",
		"bid-index":          "app-g0btm96F743Rg0BXr9Vgr8a2",
		"bid-polish":         "app-ItRvtPR9m9rkc9DLMcYUtmyX",
		"writing":            "app-0PwW0Ro1VCI1SeZHz6MI8ioG",
		"review":             "app-yUHEQLfjKxzz4FSZgdH4Nh5y",
		"review-more":        "app-UPojYSIKbk3pch2hHMGLviGZ",
		"consistent":         "app-S7z0PWGbjlfQ7FICfHxEvSGn",
		"material":           "app-hQ67g4AFrZh5x0EMg2FPBCbG",
	}
	getToken := func(k string) string {
		v := strings.TrimSpace(skbcfg.Get("agent." + k))
		if v == "" {
			v = def[k]
		}
		if strings.HasPrefix(v, "Bearer ") {
			return v
		}
		return "Bearer " + v
	}
	return map[string]*AgentConfig{
		"tender": {
			Id:          "tender",
			Name:        "招标文件字段解析",
			URL:         agentURL,
			Token:       getToken("tender"),
			Timeout:     10800,
			Description: "招标文件解析智能体服务",
		},
		"tender-trace-index": {
			Id:          "tender-trace-index",
			Name:        "招标文件字段索引智能体",
			URL:         agentURL,
			Token:       getToken("tender-trace-index"),
			Timeout:     10800,
			Description: "招标文件字段溯源索引服务。",
		},
		"bid-format": {
			Id:          "bid-format",
			Name:        "投标文件格式解析智能体",
			URL:         agentURL,
			Token:       getToken("bid-format"),
			Timeout:     10800,
			Description: "投标文件格式解析智能体服务，解析出投标章节所在页码",
		},
		"bid-index": {
			Id:          "bid-index",
			Name:        "投标文件格式解析智能体",
			URL:         agentURL,
			Token:       getToken("bid-index"),
			Timeout:     10800,
			Description: "投标文件格式解析智能体服务，解析出投标章节所在页码",
		},
		"bid-polish": {
			Id:          "bid-polish",
			Name:        "投标书文本润色智能体",
			URL:         agentURL,
			Token:       getToken("bid-polish"),
			Timeout:     3600,
			Description: "将标书的目标文本，进行专业化润色的智能体服务",
		},
		"writing": {
			Id:          "writing",
			Name:        "生成标书目录智能体",
			URL:         agentURL,
			Token:       getToken("writing"),
			Timeout:     10800,
			Description: "按用户Prompt生成标书目录智能体服务",
		},
		"review": {
			Id:          "review",
			Name:        "招投标审核智能体",
			URL:         agentURL,
			Token:       getToken("review"),
			Timeout:     10800,
			Description: "招投标审核智能体服务",
		},
		"review-more": {
			Id:          "review-more",
			Name:        "招投标评分标准审核智能体",
			URL:         agentURL,
			Token:       getToken("review-more"),
			Timeout:     10800,
			Description: "招投标评分标准审核智能体服务",
		},
		"consistent": {
			Id:          "consistent",
			Name:        "投标文件前后一致性智能体",
			URL:         agentURL,
			Token:       getToken("consistent"),
			Timeout:     10800,
			Description: "投标文件前后一致性审核智能体服务",
		},
		"material": {
			Id:          "material",
			Name:        "素材库作业SOP抽取智能体",
			URL:         agentURL,
			Token:       getToken("material"),
			Timeout:     10800,
			Description: "素材库作业SOP抽取智能体服务",
		},
		"material-image-name": {
			Id:          "material-image-name",
			Name:        "图库文件自动生成名称描述",
			URL:         agentURL,
			Token:       getToken("material-image-name"),
			Timeout:     3600,
			Description: "根据图片内容自动提炼名称和描述",
		},
	}
}
