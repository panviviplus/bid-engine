// Package sysllm 提供全局模型配置（system_llm_config）的读写与选取。
//
// 与用户级 user_llm_config 的区别：
//   - 全局模型配置没有 module 维度，是一份平铺的候选列表，由系统超管维护；
//   - 平台级后台任务（例如招标情报站的公告打标）没有用户上下文，只能读这份配置；
//   - 选取规则：字段完整（base_url / api_key / model / endpoint_path 非空）按 sort 升序，
//     sort 相同时取最新创建的一条；调用失败由调用方按序降级到下一个候选。
package sysllm

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"bid-engine/lib/common/logtool"
	"bid-engine/lib/common/storage"
	"bid-engine/pkg/db/model"
	repoLLM "bid-engine/pkg/repo/llm"
)

// ErrNoCandidate 表示没有任何可用的全局模型配置。
var ErrNoCandidate = errors.New("sysllm: 没有可用的全局模型配置")

// Service 全局模型配置读写接口。
type Service interface {
	// List 返回全部配置，按 sort 升序、id 降序。
	List(ctx context.Context) ([]*model.SystemLlmConfig, error)
	// Get 按主键查询。
	Get(ctx context.Context, id int64) (*model.SystemLlmConfig, error)
	// Create 新增配置。
	Create(ctx context.Context, item *model.SystemLlmConfig) error
	// Update 覆盖更新配置（api_key 为空表示保持原值）。
	Update(ctx context.Context, item *model.SystemLlmConfig) error
	// Delete 删除配置。
	Delete(ctx context.Context, id int64) error
	// Reorder 按传入的 id 顺序重排 sort。
	Reorder(ctx context.Context, ids []int64) error
	// Resolve 返回第一个字段完整的配置；DB 无候选时回退配置文件中的默认 provider 配置。
	Resolve(ctx context.Context) (*repoLLM.ProviderConfig, *model.SystemLlmConfig, error)
	// Candidates 返回按选取顺序排列的可用配置（已剔除字段不完整的行）。
	Candidates(ctx context.Context) ([]*model.SystemLlmConfig, error)
	// ToProviderConfig 将一行全局配置转换为运行时配置。
	ToProviderConfig(item *model.SystemLlmConfig) *repoLLM.ProviderConfig
}

// IsComplete 判断一条全局配置是否字段完整。
func IsComplete(item *model.SystemLlmConfig) bool {
	if item == nil {
		return false
	}
	return strings.TrimSpace(item.BaseURL) != "" &&
		strings.TrimSpace(item.APIKey) != "" &&
		strings.TrimSpace(item.Model) != "" &&
		strings.TrimSpace(item.EndpointPath) != ""
}

type svcImpl struct {
	db     *gorm.DB
	logger *zap.SugaredLogger
}

var (
	instance Service
	once     sync.Once
)

// GetInstance 返回全局模型配置仓储单例。
func GetInstance() Service {
	once.Do(func() {
		instance = &svcImpl{
			db:     storage.GetDB(),
			logger: logtool.GetLogger().Sugar(),
		}
	})
	return instance
}

func (s *svcImpl) List(ctx context.Context) ([]*model.SystemLlmConfig, error) {
	var items []*model.SystemLlmConfig
	err := s.db.WithContext(ctx).
		Order("sort ASC, id DESC").
		Find(&items).Error
	return items, err
}

func (s *svcImpl) Get(ctx context.Context, id int64) (*model.SystemLlmConfig, error) {
	var item model.SystemLlmConfig
	if err := s.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *svcImpl) Create(ctx context.Context, item *model.SystemLlmConfig) error {
	if item == nil {
		return errors.New("sysllm: 配置为空")
	}
	now := time.Now().Unix()
	item.CreateTime = now
	item.UpdateTime = now
	if strings.TrimSpace(item.EndpointPath) == "" {
		item.EndpointPath = "/chat/completions"
	}
	return s.db.WithContext(ctx).Create(item).Error
}

// Update 覆盖更新。api_key 为空表示沿用库中原值，避免前端拿到脱敏值后误清空。
func (s *svcImpl) Update(ctx context.Context, item *model.SystemLlmConfig) error {
	if item == nil || item.ID <= 0 {
		return errors.New("sysllm: 配置 ID 非法")
	}
	current, err := s.Get(ctx, item.ID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(item.APIKey) == "" {
		item.APIKey = current.APIKey
	}
	if strings.TrimSpace(item.EndpointPath) == "" {
		item.EndpointPath = "/chat/completions"
	}
	item.CreateTime = current.CreateTime
	item.UpdateTime = time.Now().Unix()
	return s.db.WithContext(ctx).Save(item).Error
}

func (s *svcImpl) Delete(ctx context.Context, id int64) error {
	return s.db.WithContext(ctx).Delete(&model.SystemLlmConfig{}, id).Error
}

func (s *svcImpl) Reorder(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().Unix()
		for idx, id := range ids {
			if err := tx.Model(&model.SystemLlmConfig{}).
				Where("id = ?", id).
				Updates(map[string]interface{}{
					"sort":        (idx + 1) * 10,
					"update_time": now,
				}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *svcImpl) Candidates(ctx context.Context) ([]*model.SystemLlmConfig, error) {
	items, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*model.SystemLlmConfig, 0, len(items))
	for _, item := range items {
		if IsComplete(item) {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *svcImpl) ToProviderConfig(item *model.SystemLlmConfig) *repoLLM.ProviderConfig {
	if item == nil {
		return nil
	}
	endpoint := strings.TrimSpace(item.EndpointPath)
	if endpoint == "" {
		endpoint = "/chat/completions"
	}
	contextWindow := int(item.ContextWindowTokens)
	if contextWindow <= 0 {
		contextWindow = 32768
	}
	maxOutput := int(item.MaxOutputTokens)
	if maxOutput <= 0 {
		maxOutput = 8192
	}
	return &repoLLM.ProviderConfig{
		BaseURL:             strings.TrimSpace(item.BaseURL),
		EndpointPath:        endpoint,
		APIKey:              strings.TrimSpace(item.APIKey),
		Model:               strings.TrimSpace(item.Model),
		TimeoutSeconds:      300,
		DefaultTemperature:  0.2,
		DefaultTopP:         0.9,
		DefaultMaxTokens:    maxOutput,
		ContextWindowTokens: contextWindow,
	}
}

// Resolve 返回第一个可用配置；DB 无候选时回退配置文件中的默认 provider。
func (s *svcImpl) Resolve(ctx context.Context) (*repoLLM.ProviderConfig, *model.SystemLlmConfig, error) {
	candidates, err := s.Candidates(ctx)
	if err != nil {
		return nil, nil, err
	}
	if len(candidates) > 0 {
		first := candidates[0]
		return s.ToProviderConfig(first), first, nil
	}
	// 兜底：使用配置文件中的默认 provider 配置（本地开发无 DB 配置时仍可运行）
	fallback := repoLLM.GetInstance().GetConfig(repoLLM.ProviderDeepSeek)
	if fallback == nil || strings.TrimSpace(fallback.APIKey) == "" {
		return nil, nil, ErrNoCandidate
	}
	return fallback, nil, nil
}

// SortByName 仅用于测试与调试：按 sort、id 稳定排序。
func SortByName(items []*model.SystemLlmConfig) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Sort != items[j].Sort {
			return items[i].Sort < items[j].Sort
		}
		return items[i].ID > items[j].ID
	})
}
