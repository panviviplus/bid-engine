package bidhub

const (
	// 项目状态常量定义
	ProjectStatusFailed  = "failed"  // 解析失败
	ProjectStatusSucceed = "succeed" // 已完成
	ProjectStatusRunning = "running" // 解析中

	// ===== 招标解析阶段常量（按执行顺序） =====
	TenderStageDocumentParsing    = "document_parsing"     // 文档下载/转PDF/解析
	TenderStageFieldFiltering     = "field_filtering"      // 筛选需抽取的字段
	TenderStageLLMExtracting      = "llm_extracting"       // LLM批量/逐字段抽取
	TenderStagePersistValues      = "persist_values"       // 字段值落库
	TenderStageSummary            = "summary_generation"   // 智能摘要生成
	TenderStageBidFormatExtracting = "bid_format_extracting" // 投标文件格式提取
	TenderStageTracing            = "tracing"              // 字段溯源
	TenderStageCompleted          = "completed"            // 全部完成

	// ===== 标书生成阶段常量 =====
	BidStageFileUploading = "file_uploading" // 文件上传到OSS
	BidStageProjectCreate = "project_create" // 项目记录创建
	BidStageMenuGenerate  = "menu_generate"  // AI生成目录
	BidStageBodyGenerate  = "body_generate"  // AI生成正文
	BidStageCompleted     = "completed"      // 全部完成

	// ===== 招投标审核阶段常量 =====
	ReviewStageFileUpload      = "file_upload"       // 文件上传到OSS
	ReviewStageOldDocParse     = "old_doc_parse"     // 旧招标文件解析
	ReviewStageNewDocParse     = "new_doc_parse"     // 新招标文件解析
	ReviewStageAttachmentParse = "attachment_parse"  // 附件解析
	ReviewStageContrastAgent   = "contrast_agent"    // 招文比对Agent
	ReviewStageTraceIndex      = "trace_index"       // 溯源索引
	ReviewStageCompleted       = "completed"         // 全部完成

	// ===== 素材库解析阶段常量 =====
	MaterialStageLabelGen  = "label_gen"  // 标签生成
	MaterialStageImageDesc = "image_desc" // 图片名称描述生成
	MaterialStageCompleted = "completed"  // 全部完成

	// bucket名称
	ProjectFileSaveCosBucket   = "private-1300276093" // COS 桶（云端）
	ProjectFileSaveMinioBucket = "smart-bid"          // MinIO 桶（本地开发）

	//招标文件字段来源
	TenderRecordValueSourceByModel = "model"
	TenderRecordValueSourceByUser  = "user"
)

// TenderStageOrder 招标解析阶段顺序（用于断点续跑判断）
var TenderStageOrder = []string{
	TenderStageDocumentParsing,
	TenderStageFieldFiltering,
	TenderStageLLMExtracting,
	TenderStagePersistValues,
	TenderStageSummary,
	TenderStageBidFormatExtracting,
	TenderStageTracing,
	TenderStageCompleted,
}

// ReviewStageOrder 审核阶段顺序
var ReviewStageOrder = []string{
	ReviewStageFileUpload,
	ReviewStageOldDocParse,
	ReviewStageNewDocParse,
	ReviewStageAttachmentParse,
	ReviewStageContrastAgent,
	ReviewStageTraceIndex,
	ReviewStageCompleted,
}

// ShouldSkipStage 判断断点续跑时是否应跳过已完成阶段
// resumeFrom: 从哪个阶段开始（空=从头开始）
// currentStage: 当前要执行的阶段名
func ShouldSkipStage(resumeFrom, currentStage string, stageOrder []string) bool {
	if resumeFrom == "" {
		return false // 从头跑，不跳过
	}
	foundResume := false
	for _, s := range stageOrder {
		if s == resumeFrom {
			foundResume = true
		}
		if foundResume && s == currentStage {
			return false // 到达续跑起点，不跳过
		}
		if !foundResume {
			if s == currentStage {
				return true // 在续跑起点之前，跳过
			}
		}
	}
	return false
}
