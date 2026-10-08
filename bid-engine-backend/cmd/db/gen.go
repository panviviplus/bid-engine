package main

import (
	"gorm.io/gen"

	"bid-engine/lib/common/config"
	"bid-engine/lib/common/storage"
)

// generate code
func main() {
	config.MustLoadConfig("./conf/conf-local.yml")
	storage.MustInitDB()
	g := gen.NewGenerator(gen.Config{
		OutPath:      "./pkg/db/query",
		ModelPkgPath: "./pkg/db/model",
		WithUnitTest: false,
	})

	// 初始化数据库
	g.UseDB(storage.GetDB())

	// 素材库
	g.ApplyBasic(g.GenerateModel("material_file_info"))
	g.ApplyBasic(g.GenerateModel("material_image_info"))
	g.ApplyBasic(g.GenerateModel("material_ocr_result"))
	g.ApplyBasic(g.GenerateModel("material_performance"))
	g.ApplyBasic(g.GenerateModel("material_qualification"))
	g.ApplyBasic(g.GenerateModel("material_template"))

	// 投标文件生成（旧 bid_project 表保留历史数据，继续生成以兼容存量代码）
	g.ApplyBasic(g.GenerateModel("bid_project"))
	g.ApplyBasic(g.GenerateModel("bid_gen_project",
		gen.FieldType("blueprint_generation_id", "*int64"),
	))
	g.ApplyBasic(g.GenerateModel("bid_gen_outline"))
	g.ApplyBasic(g.GenerateModel("bid_gen_chapter_content"))
	g.ApplyBasic(g.GenerateModel("bid_gen_doc_content"))
	g.ApplyBasic(g.GenerateModel("bid_gen_task",
		gen.FieldType("queue_task_id", "*string"),
		gen.FieldType("started_at", "*time.Time"),
		gen.FieldType("heartbeat_at", "*time.Time"),
		gen.FieldType("cancel_requested_at", "*time.Time"),
		gen.FieldType("finished_at", "*time.Time"),
	))
	g.ApplyBasic(g.GenerateModel("bid_gen_material_ref"))
	g.ApplyBasic(g.GenerateModel("bid_gen_export_record"))
	g.ApplyBasic(g.GenerateModel("bid_gen_source_snapshot"))
	g.ApplyBasic(g.GenerateModel("bid_gen_chapter_spec"))

	// 招标文件解析V3（全文分块、运行态、事实与多证据溯源）
	g.ApplyBasic(g.GenerateModel("bid_analysis_v3_project"))
	g.ApplyBasic(g.GenerateModel("bid_analysis_v3_project_deleted"))
	g.ApplyBasic(g.GenerateModel("bid_analysis_v3_parse_run",
		gen.FieldType("completed_at", "*time.Time"),
		gen.FieldType("activated_at", "*time.Time"),
	))
	g.ApplyBasic(g.GenerateModel("bid_analysis_v3_run_control",
		gen.FieldType("applied_at", "*time.Time"),
	))
	g.ApplyBasic(g.GenerateModel("bid_analysis_v3_stage_run",
		gen.FieldType("started_at", "*time.Time"),
		gen.FieldType("completed_at", "*time.Time"),
	))
	g.ApplyBasic(g.GenerateModel("bid_analysis_v3_stage_task",
		gen.FieldType("started_at", "*time.Time"),
		gen.FieldType("completed_at", "*time.Time"),
	))
	g.ApplyBasic(g.GenerateModel("bid_analysis_v3_warning",
		gen.FieldType("detail_json", "*string"),
	))
	g.ApplyBasic(g.GenerateModel("bid_analysis_v3_operation_log"))
	g.ApplyBasic(g.GenerateModel("bid_analysis_v3_document_asset"))
	g.ApplyBasic(g.GenerateModel("bid_analysis_v3_document_chunk"))
	g.ApplyBasic(g.GenerateModel("bid_analysis_v3_document_page"))
	g.ApplyBasic(g.GenerateModel("bid_analysis_v3_document_block"))
	g.ApplyBasic(g.GenerateModel("bid_analysis_v3_source_table"))
	g.ApplyBasic(g.GenerateModel("bid_analysis_v3_chapter"))
	g.ApplyBasic(g.GenerateModel("bid_analysis_v3_chapter_block"))
	g.ApplyBasic(g.GenerateModel("bid_analysis_v3_llm_call"))
	g.ApplyBasic(g.GenerateModel("bid_analysis_v3_field_category"))
	g.ApplyBasic(g.GenerateModel("bid_analysis_v3_field_catalog"))
	g.ApplyBasic(g.GenerateModel("bid_analysis_v3_fact_candidate"))
	g.ApplyBasic(g.GenerateModel("bid_analysis_v3_field"))
	g.ApplyBasic(g.GenerateModel("bid_analysis_v3_field_value"))
	g.ApplyBasic(g.GenerateModel("bid_analysis_v3_field_value_evidence"))
	g.ApplyBasic(g.GenerateModel("bid_analysis_v3_derived_table"))
	g.ApplyBasic(g.GenerateModel("bid_analysis_v3_derived_table_evidence"))
	g.ApplyBasic(g.GenerateModel("bid_analysis_v3_clause"))
	g.ApplyBasic(g.GenerateModel("bid_analysis_v3_clause_evidence"))
	g.ApplyBasic(g.GenerateModel("bid_analysis_v3_summary"))
	g.ApplyBasic(g.GenerateModel("bid_analysis_v3_summary_risk_resolution",
		gen.FieldType("resolved_at", "*time.Time"),
		gen.FieldType("resolved_by", "*int64"),
	))
	g.ApplyBasic(g.GenerateModel("bid_analysis_v3_follow"))
	g.ApplyBasic(g.GenerateModel("bid_analysis_v3_blueprint"))
	g.ApplyBasic(g.GenerateModel("bid_analysis_v3_blueprint_generation",
		gen.FieldType("started_at", "*time.Time"),
		gen.FieldType("completed_at", "*time.Time"),
	))
	g.ApplyBasic(g.GenerateModel("bid_analysis_v3_deletion_job",
		gen.FieldType("completed_at", "*time.Time"),
	))
	g.ApplyBasic(g.GenerateModel("bid_analysis_v3_deletion_object"))

	// 投标书审核（bid-audit）
	g.ApplyBasic(g.GenerateModel("bid_review_project"))
	g.ApplyBasic(g.GenerateModel("bid_review_file"))
	g.ApplyBasic(g.GenerateModel("bid_review_check_item", gen.FieldType("reviewed_at", "*time.Time")))
	g.ApplyBasic(g.GenerateModel("bid_review_op_log"))
	g.ApplyBasic(g.GenerateModel("bid_review_export_record"))

	// 企业模板
	g.ApplyBasic(g.GenerateModel("enterprise_template"))

	// 用户与公司
	g.ApplyBasic(g.GenerateModel("company"))
	g.ApplyBasic(g.GenerateModel("user"))
	g.ApplyBasic(g.GenerateModel("feishu_account_binding"))
	g.ApplyBasic(g.GenerateModel("user_extra_profile"))
	g.ApplyBasic(g.GenerateModel("user_llm_config"))

	// KV 存储
	g.ApplyBasic(g.GenerateModel("kv"))

	// 反馈管理
	g.ApplyBasic(g.GenerateModel("feedback_record"))

	// 任务队列
	g.ApplyBasic(g.GenerateModel("task_queue"))

	// 审核记录
	g.ApplyBasic(g.GenerateModel("audit"))

	// 全局模型配置（平台级后台任务，例如招标情报站）
	g.ApplyBasic(g.GenerateModel("system_llm_config"))

	// 招标情报站
	g.ApplyBasic(g.GenerateModel("tender_intel_industry"))
	g.ApplyBasic(g.GenerateModel("tender_intel_source"))
	g.ApplyBasic(g.GenerateModel("tender_intel_collect_schedule"))
	g.ApplyBasic(g.GenerateModel("tender_intel_collect_run",
		gen.FieldType("finished_at", "*time.Time"),
	))
	g.ApplyBasic(g.GenerateModel("tender_intel_run_source"))
	g.ApplyBasic(g.GenerateModel("tender_intel_notice",
		gen.FieldType("budget_amount", "*float64"),
		gen.FieldType("publish_date", "*time.Time"),
		gen.FieldType("publish_at", "*time.Time"),
		gen.FieldType("deadline_at", "*time.Time"),
		gen.FieldType("pinned_at", "*time.Time"),
	))
	g.ApplyBasic(g.GenerateModel("tender_intel_notice_industry"))
	g.ApplyBasic(g.GenerateModel("tender_intel_notice_insight"))
	g.ApplyBasic(g.GenerateModel("tender_intel_subscription",
		gen.FieldType("budget_min", "*float64"),
		gen.FieldType("budget_max", "*float64"),
		gen.FieldType("last_matched_at", "*time.Time"),
		gen.FieldType("backfilled_at", "*time.Time"),
	))
	g.ApplyBasic(g.GenerateModel("tender_intel_alert",
		gen.FieldType("read_at", "*time.Time"),
		gen.FieldType("feishu_push_next_at", "*time.Time"),
		gen.FieldType("feishu_push_sent_at", "*time.Time"),
	))
	g.ApplyBasic(g.GenerateModel("tender_intel_favorite"))
	g.ApplyBasic(g.GenerateModel("tender_intel_match_task",
		gen.FieldType("range_from", "*time.Time"),
		gen.FieldType("range_to", "*time.Time"),
		gen.FieldType("started_at", "*time.Time"),
		gen.FieldType("finished_at", "*time.Time"),
	))

	g.Execute()
}
