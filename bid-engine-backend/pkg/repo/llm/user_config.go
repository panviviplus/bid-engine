package llm

import (
	"context"
	"strconv"
	"strings"
	"sync"

	"bid-engine/lib/common/storage"
	"bid-engine/pkg/db/model"
	"bid-engine/pkg/db/query"
)

// ── context key ──────────────────────────────────────────────────

type ctxKeyUserID struct{}

// WithUserID 将用户 ID 注入 context，供 ResolveConfig 读取
func WithUserID(ctx context.Context, userID int64) context.Context {
	return context.WithValue(ctx, ctxKeyUserID{}, userID)
}

func getUserIDFromCtx(ctx context.Context) int64 {
	if v, ok := ctx.Value(ctxKeyUserID{}).(int64); ok {
		return v
	}
	return 0
}

// ── module 常量枚举（user_llm_config.module 字段）─────────────────
// 新功能模块扩展时，在此追加常量并同步 AllModules / featureToModule

const (
	ModuleAll           = "all"            // 全局配置
	ModuleBidAnalysis   = "bid_analysis"   // 招标解析
	ModuleBidGeneration = "bid_generation" // 标书生成
	ModuleBidReview     = "bid_review"     // 标书审核
	ModuleMaterial      = "material"       // 素材库
)

// ── feature → module 映射 ────────────────────────────────────────

var featureToModule = map[string]string{
	// 招标解析
	"bid_analysis_chapter_identify":      ModuleBidAnalysis,
	"bid_analysis_core_field_extract":    ModuleBidAnalysis,
	"bid_analysis_clause_extract":        ModuleBidAnalysis,
	"bid_analysis_risk_assess":           ModuleBidAnalysis,
	"bid_analysis_score_estimate":        ModuleBidAnalysis,
	"bid_analysis_blueprint":             ModuleBidAnalysis,
	"bid_analysis_project_name_fallback": ModuleBidAnalysis,
	"bid_analysis_fact_extract":          ModuleBidAnalysis,
	"bid_analysis_fact_consolidate":      ModuleBidAnalysis,
	"bid_analysis_ai_interpret":          ModuleBidAnalysis,
	"tender_analysis_field_extract":      ModuleBidAnalysis,
	"tender_analysis_bid_format":         ModuleBidAnalysis,
	"tender_analysis_batch_extract":      ModuleBidAnalysis,
	"tender_analysis_summary":            ModuleBidAnalysis,
	// 标书生成
	"bid_generation":           ModuleBidGeneration,
	"bid_menu_generation":      ModuleBidGeneration,
	"bid_text_polish":          ModuleBidGeneration,
	"bid_gen_chapter_write":    ModuleBidGeneration,
	"bid_gen_chapter_spec":     ModuleBidGeneration,
	"bid_gen_template_outline": ModuleBidGeneration,
	// 标书审核（bid-audit）
	"bid_review_checklist":      ModuleBidReview,
	"bid_review_tender_extract": ModuleBidReview,
	"bid_review_verdict":        ModuleBidReview,
	"bid_review_verdict_batch":  ModuleBidReview,
	"bid_review_scoring":        ModuleBidReview,
	// 素材库
	"material_ocr_extract": ModuleMaterial,
	"material_description": ModuleMaterial,
}

// AllModules 所有模块标识（包含全局 "all"）
var AllModules = []string{
	ModuleAll,
	ModuleBidAnalysis,
	ModuleBidGeneration,
	ModuleBidReview,
	ModuleMaterial,
}

// BusinessModules 业务功能模块（不含全局 "all"），供模块列表下发与合法性校验
var BusinessModules = []string{
	ModuleBidAnalysis,
	ModuleBidGeneration,
	ModuleBidReview,
	ModuleMaterial,
}

// ModuleNames 模块中文名（与前端左侧菜单对齐）
var ModuleNames = map[string]string{
	ModuleAll:           "全局配置",
	ModuleBidAnalysis:   "招标解析",
	ModuleBidGeneration: "投标文件生成",
	ModuleBidReview:     "投标文件审核",
	ModuleMaterial:      "素材库",
}

// IsModuleValid 校验模块标识是否合法（全局 all 或业务模块）
func IsModuleValid(module string) bool {
	module = strings.TrimSpace(module)
	if module == ModuleAll {
		return true
	}
	for _, m := range BusinessModules {
		if m == module {
			return true
		}
	}
	return false
}

// ── 缓存 ─────────────────────────────────────────────────────────

var (
	configCache sync.Map // key: "userID:module" → *queryResult
)

type queryResult struct {
	cfg *model.UserLlmConfig
	err error
}

// InvalidateCache 清除指定用户的所有配置缓存（PUT 保存后调用）
func InvalidateCache(userID int64) {
	for _, mod := range AllModules {
		configCache.Delete(cacheKey(userID, mod))
	}
}

func cacheKey(userID int64, module string) string {
	return strconv.FormatInt(userID, 10) + ":" + module
}

func loadUserConfig(ctx context.Context, userID int64, module string) *model.UserLlmConfig {
	key := cacheKey(userID, module)
	if v, ok := configCache.Load(key); ok {
		if r, ok2 := v.(*queryResult); ok2 {
			if r.err == nil {
				return r.cfg
			}
			// 之前查询出错，驱逐缓存后重试
			configCache.Delete(key)
		}
	}
	q := query.Use(storage.GetDB())
	cfg, err := q.UserLlmConfig.WithContext(ctx).
		Where(q.UserLlmConfig.UserID.Eq(userID), q.UserLlmConfig.Module.Eq(module)).
		First()
	configCache.Store(key, &queryResult{cfg: cfg, err: err})
	return cfg
}

// ── ResolveConfig ────────────────────────────────────────────────

// ResolveConfig 解析 feature 对应模块的用户 LLM 配置（优先级链：子模块 > 全局 > nil）。
// 仅认用户配置：返回 nil 表示用户未配置，业务层应直接报错（ErrLLMNotConfigured），不再回退系统 llm-config.yml。
func ResolveConfig(ctx context.Context, feature string) *ProviderConfig {
	module, ok := featureToModule[feature]
	if !ok {
		return nil
	}
	return ResolveModuleConfig(ctx, module)
}

// moduleConfigComplete 模块配置是否完整：base_url / api_key / model / endpoint_path 均非空才算有效配置。
// 整条配置级回退策略下，残缺的模块行视为“该模块未配置”，整体回退 module=all。
func moduleConfigComplete(cfg *model.UserLlmConfig) bool {
	if cfg == nil {
		return false
	}
	return strings.TrimSpace(cfg.BaseURL) != "" &&
		strings.TrimSpace(cfg.APIKey) != "" &&
		strings.TrimSpace(cfg.Model) != "" &&
		strings.TrimSpace(cfg.EndpointPath) != ""
}

// toProviderConfig 将 user_llm_config 行转为运行时 ProviderConfig（补默认值）。
func toProviderConfig(cfg *model.UserLlmConfig) *ProviderConfig {
	if cfg == nil {
		return nil
	}
	endpoint := strings.TrimSpace(cfg.EndpointPath)
	if endpoint == "" {
		endpoint = "/chat/completions"
	}
	contextWindow := int(cfg.ContextWindowTokens)
	if contextWindow <= 0 {
		contextWindow = 32768
	}
	maxOutput := int(cfg.MaxOutputTokens)
	if maxOutput <= 0 {
		maxOutput = 8192
	}
	return &ProviderConfig{
		BaseURL:             strings.TrimSpace(cfg.BaseURL),
		EndpointPath:        endpoint,
		APIKey:              strings.TrimSpace(cfg.APIKey),
		Model:               strings.TrimSpace(cfg.Model),
		TimeoutSeconds:      300,
		DefaultTemperature:  0.2,
		DefaultTopP:         0.9,
		DefaultMaxTokens:    maxOutput,
		ContextWindowTokens: contextWindow,
	}
}

// resolveModuleConfigFromRows 整条配置级回退（纯函数，便于单测）：
//   - mod 行完整 → 用 mod 整条；
//   - 否则 all 行完整 → 用 all 整条；
//   - 否则返回 nil（未配置）。
func resolveModuleConfigFromRows(modCfg, allCfg *model.UserLlmConfig) *ProviderConfig {
	if moduleConfigComplete(modCfg) {
		return toProviderConfig(modCfg)
	}
	if moduleConfigComplete(allCfg) {
		return toProviderConfig(allCfg)
	}
	return nil
}

// ResolveModuleConfig 解析指定模块的用户 LLM 配置（整条配置级回退：模块 → 全局 → nil）。
// module 为空时仅解析 module=all（全局）配置。
func ResolveModuleConfig(ctx context.Context, module string) *ProviderConfig {
	userID := getUserIDFromCtx(ctx)
	if userID == 0 {
		return nil
	}
	mod := strings.TrimSpace(module)
	if mod == "" {
		mod = ModuleAll
	}

	var modCfg *model.UserLlmConfig
	if mod != ModuleAll {
		modCfg = loadUserConfig(ctx, userID, mod)
	}
	allCfg := loadUserConfig(ctx, userID, ModuleAll)

	return resolveModuleConfigFromRows(modCfg, allCfg)
}

// ── API Key 脱敏 ─────────────────────────────────────────────────

// MaskAPIKey 脱敏：前 2 + **** + 后 3。原值 < 7 位全掩。
func MaskAPIKey(raw string) string {
	if raw == "" {
		return ""
	}
	if len(raw) < 7 {
		return "****"
	}
	return raw[:2] + "****" + raw[len(raw)-3:]
}

// IsMasked 判断是否为脱敏值（含 **** 掩码）
func IsMasked(s string) bool {
	return strings.Contains(s, "****")
}
