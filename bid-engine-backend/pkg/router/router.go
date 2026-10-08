package router

import (
	"github.com/gin-gonic/gin"

	"bid-engine/pkg/handler/bidanalysisv3"
	"bid-engine/pkg/handler/bidgen"
	"bid-engine/pkg/handler/bidhub"
	"bid-engine/pkg/handler/bidreview"
	"bid-engine/pkg/handler/feedback"
	feishuHandler "bid-engine/pkg/handler/feishu"
	homeHandler "bid-engine/pkg/handler/home"
	"bid-engine/pkg/handler/llmconfig"
	materialHandler "bid-engine/pkg/handler/material"

	"bid-engine/pkg/handler/admin"
	"bid-engine/pkg/handler/open"
	sysllmHandler "bid-engine/pkg/handler/sysllm"
	tenderintelHandler "bid-engine/pkg/handler/tenderintel"
	"bid-engine/pkg/handler/user"
	"bid-engine/pkg/middleware"
)

var (
	auth     = middleware.AuthMiddleware()
	openAuth = middleware.OpenAuth()
)

// RegisterRouter 注册路由
func RegisterRouter(e *gin.Engine) {

	api := e.Group("/api/")

	// 用户相关
	registerUser(e)
	registerUser(api)
	registerFeishu(e)
	registerFeishu(api)

	// 管理后台
	registerAdmin(e)
	registerAdmin(api)

	// 首页聚合数据
	registerHome(e)
	registerHome(api)

	// 开放平台
	registerOpen(e)
	registerOpen(api)

	// 招标解析模块
	registerSmartBid(e)
	registerSmartBid(api)

	// 标书生成模块
	registerBidGen(e)
	registerBidGen(api)

	// 投标书审核模块
	registerBidReview(e)
	registerBidReview(api)

	// 反馈管理
	registerFeedback(e)
	registerFeedback(api)

	// 素材库
	registerMaterial(e)
	registerMaterial(api)

	// 系统管理-模型配置（独立功能模块）
	registerLLMConfig(e)
	registerLLMConfig(api)

	// 招标情报站
	registerTenderIntel(e)
	registerTenderIntel(api)

	// 系统管理-系统模型配置（全局模型配置，仅超管）
	registerSystemLLMConfig(e)
	registerSystemLLMConfig(api)
}

func registerFeishu(e gin.IRouter) {
	s := feishuHandler.GetInstance()
	e.POST("/feishu/login/challenge", s.LoginChallenge)
	e.GET("/feishu/login/challenge", s.LoginStatus)
	e.POST("/feishu/login/complete", s.LoginComplete)
	e.POST("/user/feishu/bind/challenge", auth, s.BindingChallenge)
	e.GET("/user/feishu/bind/challenge", auth, s.BindingStatus)
	e.POST("/user/feishu/bind/complete", auth, s.BindingComplete)
	e.GET("/user/feishu", auth, s.BindingInfo)
	e.DELETE("/user/feishu", auth, s.Unbind)
	e.PUT("/user/feishu/notify", auth, s.SetNotify)
	e.PUT("/user/feishu/profile", auth, s.UpdateExtraProfile)
}

// registerTenderIntel 招标情报站业务接口
func registerTenderIntel(e gin.IRouter) {
	s := tenderintelHandler.GetInstance()

	// ===== 情报大厅 =====
	e.GET("/zb/intel/notices", auth, s.ListNotices)
	e.GET("/zb/intel/notices/:id", auth, s.GetNotice)
	e.POST("/zb/intel/notices/:id/favorite", auth, s.SetFavorite)
	e.GET("/zb/intel/filters", auth, s.Filters)
	e.POST("/zb/intel/notices/:id/insight", auth, s.GenerateInsight)
	e.POST("/zb/intel/notices/:id/parse-link", auth, s.ParseLink)

	// ===== 订阅与提醒 =====
	e.GET("/zb/intel/subscriptions", auth, s.ListSubscriptions)
	e.POST("/zb/intel/subscriptions", auth, s.CreateSubscription)
	e.PUT("/zb/intel/subscriptions/:id", auth, s.UpdateSubscription)
	e.DELETE("/zb/intel/subscriptions/:id", auth, s.DeleteSubscription)
	e.POST("/zb/intel/subscriptions/enable", auth, s.SetSubscriptionEnabled)
	e.POST("/zb/intel/subscriptions/parse", auth, s.ParseSubscription)
	e.GET("/zb/intel/alerts", auth, s.ListAlerts)
	e.POST("/zb/intel/alerts/status", auth, s.SetAlertsStatus)
	e.POST("/zb/intel/alerts/delete", auth, s.DeleteAlerts)
	e.GET("/zb/intel/alerts/unread-count", auth, s.UnreadCount)

	// ===== 采集运维（仅超管）=====
	e.GET("/zb/intel/sources", auth, s.ListSources)
	e.GET("/zb/intel/sources/template", auth, s.DownloadSourceTemplate)
	e.POST("/zb/intel/sources/import", auth, s.ImportSources)
	e.PUT("/zb/intel/sources/:key", auth, s.UpdateSource)
	e.DELETE("/zb/intel/sources/:key", auth, s.DeleteSource)
	e.POST("/zb/intel/sources/:key/probe", auth, s.ProbeSource)
	e.GET("/zb/intel/runs", auth, s.ListRuns)
	e.GET("/zb/intel/runs/:runId", auth, s.GetRunDetail)
	e.POST("/zb/intel/runs/:runId/retry", auth, s.RetryRun)
	e.DELETE("/zb/intel/runs/:runId", auth, s.DeleteRun)
	e.POST("/zb/intel/collect/run", auth, s.TriggerCollect)

	// ===== 自动采集任务配置（仅超管）=====
	e.GET("/zb/intel/schedule", auth, s.GetSchedule)
	e.PUT("/zb/intel/schedule", auth, s.UpdateSchedule)

	// ===== 订阅匹配作业面（仅超管）=====
	e.GET("/zb/intel/matches/overview", auth, s.MatchOverview)
	e.GET("/zb/intel/matches/tasks", auth, s.ListMatchTasks)
	e.GET("/zb/intel/matches/tasks/:taskNo", auth, s.GetMatchTask)
	e.PUT("/zb/intel/matches/tasks/:taskNo/priority", auth, s.SetMatchTaskPriority)
	e.POST("/zb/intel/matches/tasks/:taskNo/cancel", auth, s.CancelMatchTask)
	e.POST("/zb/intel/matches/tasks/:taskNo/retry", auth, s.RetryMatchTask)
	e.DELETE("/zb/intel/matches/tasks/:taskNo", auth, s.DeleteMatchTask)
	e.POST("/zb/intel/matches/rescan", auth, s.RescanMatches)

	// ===== 情报管理（仅超管）=====
	e.GET("/zb/intel/admin/notices", auth, s.ListAdminNotices)
	e.POST("/zb/intel/admin/notices", auth, s.CreateAdminNotice)
	e.PUT("/zb/intel/admin/notices", auth, s.UpdateAdminNotice)
	e.POST("/zb/intel/admin/notices/status", auth, s.SetNoticeStatus)
	e.POST("/zb/intel/admin/notices/pin", auth, s.SetNoticePinned)
	e.POST("/zb/intel/admin/notices/delete", auth, s.DeleteAdminNotices)
	e.GET("/zb/intel/admin/notices/template", auth, s.DownloadNoticeTemplate)
	e.POST("/zb/intel/admin/notices/import", auth, s.ImportNotices)
}

// registerSystemLLMConfig 系统管理-系统模型配置（全局模型配置）
func registerSystemLLMConfig(e gin.IRouter) {
	s := sysllmHandler.GetInstance()
	e.GET("/sys/llm-config", auth, s.List)
	e.POST("/sys/llm-config", auth, s.Create)
	e.PUT("/sys/llm-config/:id", auth, s.Update)
	e.DELETE("/sys/llm-config/:id", auth, s.Delete)
	// 顺序调整单独挂在静态路径上，避免与 /:id 通配路由冲突
	e.PUT("/sys/llm-config-order", auth, s.Reorder)
	e.POST("/sys/llm-config/test", auth, s.TestConnection)
	e.GET("/sys/llm-config/resolve", auth, s.Resolve)
}

// registerHome 首页专用接口（个人工作台）
func registerHome(e gin.IRouter) {
	h := homeHandler.GetInstance()
	e.GET("/home/stats", auth, h.Stats)
	e.GET("/home/recent-work", auth, h.RecentWork)
	e.GET("/home/attention", auth, h.Attention)
	e.GET("/home/llm-config-status", auth, h.LLMConfigStatus)
}

// registerLLMConfig 系统管理-模型配置（独立功能模块）
func registerLLMConfig(e gin.IRouter) {
	lc := llmconfig.GetInstance()
	e.GET("/system/modules", auth, lc.Modules)                   // 业务功能模块列表
	e.GET("/system/llm-config", auth, lc.Get)                    // 获取用户LLM配置（脱敏）
	e.PUT("/system/llm-config/:module", auth, lc.SaveModule)     // 保存单个模块配置（all 或业务模块）
	e.DELETE("/system/llm-config/:module", auth, lc.ClearModule) // 清空单个模块配置
	e.DELETE("/system/llm-config", auth, lc.Clear)               // 清空用户全部LLM配置
	e.POST("/system/test-ai", auth, lc.TestConnection)           // 测试提交的LLM配置是否可用
	e.GET("/system/llm-config/exist", auth, lc.Exist)            // 校验模块LLM配置是否存在（无LLM调用）
}

// registerSmartBid 招标解析模块业务接口
func registerSmartBid(e gin.IRouter) {
	s := bidhub.GetInstance()

	// =============================  Dashboard 统计接口  =============================
	e.GET("/zb/stats/analysis", auth, s.GetAnalysisStats)     // 招标解析（V2）统计（Dashboard）
	e.GET("/zb/stats/generation", auth, s.GetGenerationStats) // 投标生成统计（Dashboard）

	// =============================  招标解析 V3  =============================
	v3 := bidanalysisv3.GetInstance()
	e.POST("/zb/v3/projects", auth, v3.CreateProject)
	e.GET("/zb/v3/projects", auth, v3.ListProjects)
	e.GET("/zb/v3/projects/:id", auth, v3.GetProject)
	e.GET("/zb/v3/projects/:id/progress", auth, v3.GetProgress)
	e.POST("/zb/v3/projects/:id/reparse", auth, v3.Reparse)
	e.POST("/zb/v3/projects/:id/retry", auth, v3.Retry)
	e.POST("/zb/v3/projects/:id/retry-stage", auth, v3.RetryStage)
	e.PATCH("/zb/v3/projects/:id/warnings/resolve", auth, v3.ResolveWarnings)
	e.PATCH("/zb/v3/projects/:id/summary-risks/:index", auth, v3.ResolveSummaryRisk)
	e.POST("/zb/v3/projects/:id/follows", auth, v3.AddFollow)
	e.DELETE("/zb/v3/projects/:id/follows/:followId", auth, v3.RemoveFollow)
	e.POST("/zb/v3/projects/:id/pause", auth, v3.Pause)
	e.POST("/zb/v3/projects/:id/resume", auth, v3.Resume)
	e.POST("/zb/v3/projects/:id/skip-stage", auth, v3.SkipCurrentStage)
	e.DELETE("/zb/v3/projects/:id", auth, v3.DeleteProject)
	e.PATCH("/zb/v3/field-values/:id", auth, v3.UpdateFieldValue)
	e.POST("/zb/v3/field-values/:id/evidences", auth, v3.AddEvidences)
	e.GET("/zb/v3/field-values/:id/evidences", auth, v3.GetEvidences)
	e.GET("/zb/v3/projects/:id/source-pdf", auth, v3.GetSourcePDF)
	e.GET("/zb/v3/source-tables/:id", auth, v3.GetSourceTable)
	e.GET("/zb/v3/projects/:id/blueprint", auth, v3.GetBlueprint)
	e.POST("/zb/v3/projects/:id/blueprint", auth, v3.CreateBlueprint)
	e.POST("/zb/v3/projects/:id/blueprint/retry", auth, v3.RetryBlueprint)
	e.POST("/zb/v3/projects/:id/blueprint/nodes", auth, v3.AddBlueprintNode)
	e.PATCH("/zb/v3/blueprint-nodes/:nodeId", auth, v3.UpdateBlueprintNode)
	e.POST("/zb/v3/blueprint-nodes/:nodeId/move", auth, v3.MoveBlueprintNode)
	e.DELETE("/zb/v3/blueprint-nodes/:nodeId", auth, v3.DeleteBlueprintNode)
	e.POST("/zb/v3/blueprint-nodes/:nodeId/adopt", auth, v3.AdoptBlueprintNode)
	e.POST("/zb/v3/blueprint-nodes/:nodeId/remove", auth, v3.RemoveBlueprintSuggestion)
	e.POST("/zb/v3/projects/:id/blueprint/create-bid", auth, v3.CreateBidFromBlueprint)
}

// 素材库接口
func registerMaterial(e gin.IRouter) {
	m := materialHandler.GetInstance()
	e.GET("/material/types", auth, m.Types)
	e.GET("/material/companies", auth, m.Companies)
	e.GET("/material/users", auth, m.Users)
	e.GET("/material/list", auth, m.List)
	e.GET("/material/gallery", auth, m.Gallery)
	e.DELETE("/material/gallery/images", auth, m.GalleryDelete)
	e.PUT("/material/gallery/move", auth, m.GalleryMove)
	e.POST("/material/gallery/upload", auth, m.GalleryUpload)
	e.POST("/material/file/add/:id", auth, m.AddFile)
	e.DELETE("/material/file/delete/:id", auth, m.DeleteFile)
	e.GET("/material/file/download/*objectKey", auth, m.DownloadFile)
	e.GET("/material/file/preview/:id", auth, m.PreviewFile)
	e.PUT("/material/image/edit", auth, m.EditImage)
	e.DELETE("/material/image/delete/:id", auth, m.DeleteImage)
	e.PUT("/material/image/sort-order", auth, m.SortImageFiles)
	e.PUT("/material/file/sort-order", auth, m.SortDocFiles)

	// 素材 OCR 解析
	e.GET("/material/ocr-results", auth, m.GetOcrResults)
	e.POST("/material/ocr/retry/:id", auth, m.RetryOcr)

	// 类型专属路由 — 企业资质
	e.GET("/material/qualification/list", auth, m.ListQualifications)
	e.POST("/material/qualification/add", auth, m.AddQualification)
	e.GET("/material/qualification/detail/:id", auth, m.DetailQualification)
	e.PUT("/material/qualification/update/:id", auth, m.UpdateQualification)
	e.DELETE("/material/qualification/delete/:id", auth, m.DeleteQualification)
	// 类型专属路由 — 企业业绩
	e.GET("/material/performance/list", auth, m.ListPerformances)
	e.POST("/material/performance/add", auth, m.AddPerformance)
	e.GET("/material/performance/detail/:id", auth, m.DetailPerformance)
	e.PUT("/material/performance/update/:id", auth, m.UpdatePerformance)
	e.DELETE("/material/performance/delete/:id", auth, m.DeletePerformance)
	// 类型专属路由 — 文档模板
	e.GET("/material/template/list", auth, m.ListTemplates)
	e.POST("/material/template/add", auth, m.AddTemplate)
	e.GET("/material/template/detail/:id", auth, m.DetailTemplate)
	e.PUT("/material/template/update/:id", auth, m.UpdateTemplate)
	e.DELETE("/material/template/delete/:id", auth, m.DeleteTemplate)
}

// registerUser 用户相关
func registerUser(e gin.IRouter) {
	u := user.GetInstance()
	e.GET("/info", auth, u.Info)
	e.GET("/login", u.Login)
	e.POST("/login", u.Login)                          // 登录，本地登录
	e.POST("/sms/send", u.SendSmsCode)                 // 发送短信验证码（本地验证码存储）
	e.POST("/register", u.Register)                    // 手机号注册
	e.GET("/logout", u.Logout)                         // 退出登录
	e.PUT("/user/profile", auth, u.UpdateProfile)      // 编辑当前用户资料
	e.PUT("/user/password", auth, u.ChangePassword)    // 修改登录密码
	e.GET("/user/avatar", auth, u.GetAvatar)           // 获取头像文件
	e.POST("/user/avatar", auth, u.UploadAvatar)       // 上传头像文件
	e.GET("/authLogin", u.AuthLogin)                   // IAM单点登录回调
	e.GET("/user/iam/:sn", auth, u.GetUserBySNFromIAM) // 根据SN码取IAM用户信息

	/*********** 系统管理常规接口 ***********/
	//获取同一个公司的用户列表，不分页
	e.GET("/company/userList", auth, u.ListCompanyUser)
}

func registerAdmin(e gin.IRouter) {
	adm := admin.GetInstance()
	// 公司管理
	e.GET("/adm/companyList", auth, adm.ListCompanyWithoutPage) //不分页，获取公司列表
	e.GET("/adm/company/detail", auth, adm.CompanyDetail)
	e.GET("/adm/company/list", auth, adm.ListCompany)
	e.POST("/adm/company/addOwner", auth, adm.AddCompanyOwner) //页面指定公司负责人
	e.DELETE("/adm/company/:company_id", auth, adm.DeleteCompany)
	e.PUT("/adm/company/:company_id", auth, adm.EditCompany)

	// 用户管理
	e.GET("/adm/userList", auth, adm.ListUserWithoutPage) //不分页，获取所有用户
	e.PUT("/adm/user/:user_id", auth, adm.UpdateUser)
}

// 反馈管理
func registerFeedback(e gin.IRouter) {
	s := feedback.GetInstance()
	e.GET("/feedback/options", auth, s.Options)                       // 筛选项（类型列表）
	e.POST("/feedback/record", auth, s.AddRecord)                     // 新增
	e.GET("/feedback/records", auth, s.SearchRecords)                 // 列表查询（分页）
	e.GET("/feedback/record/:record_id", auth, s.GetRecord)           // 单条详情
	e.PUT("/feedback/record/:record_id", auth, s.UpdateRecord)        // 更新
	e.PUT("/feedback/record/:record_id/status", auth, s.UpdateStatus) // 更新处理状态（仅系统管理员）
	e.DELETE("/feedback/record/:record_id", auth, s.DeleteRecord)     // 删除
	e.GET("/feedback/image", auth, s.GetImageByKey)                   // 根据对象key获取图片
}

// registerOpen 开放接口
func registerOpen(e gin.IRouter) {
	o := open.GetInstance()
	e.GET("/open/users", openAuth, o.GetUsers)
}

// registerBidGen 标书生成模块
func registerBidGen(e gin.IRouter) {
	g := bidgen.GetInstance()
	// ===== 项目管理 =====
	e.POST("/zb/file-gen/project/create", auth, g.CreateProject)                    // 创建空白标书
	e.POST("/zb/file-gen/project/create-from-tender", auth, g.CreateFromTender)     // 从招标文件创建
	e.POST("/zb/file-gen/project/create-from-template", auth, g.CreateFromTemplate) // 从模板创建
	e.POST("/zb/file-gen/project/confirm-outline", auth, g.ConfirmOutline)          // 大纲确认
	e.GET("/zb/file-gen/project/list", auth, g.PageListProject)                     // 项目列表
	e.GET("/zb/file-gen/project/detail", auth, g.GetProjectDetail)                  // 项目详情
	e.DELETE("/zb/file-gen/project/delete", auth, g.DeleteProject)                  // 删除项目
	e.DELETE("/zb/file-gen/project/delete/batch", auth, g.BatchDeleteProject)       // 批量删除项目
	e.PUT("/zb/file-gen/project/save", auth, g.SaveDocContent)                      // 自动保存文档

	// ===== 大纲 =====
	e.GET("/zb/file-gen/outline", auth, g.GetOutline)                    // 大纲树
	e.POST("/zb/file-gen/outline/add", auth, g.AddOutlineNode)           // 添加大纲节点
	e.PUT("/zb/file-gen/outline/update", auth, g.UpdateOutlineNode)      // 更新大纲节点
	e.DELETE("/zb/file-gen/outline/delete", auth, g.DeleteOutlineNode)   // 删除大纲节点
	e.POST("/zb/file-gen/outline/apply", auth, g.ApplyOutline)           // 大纲结构快照（排序/层级，单事务）
	e.POST("/zb/file-gen/outline/sync", auth, g.SyncOutline)             // 文档标题结构同步大纲
	e.POST("/zb/file-gen/outline/unconfirm", auth, g.UnconfirmOutline)   // 撤销大纲确认（draft → outline_review）
	e.POST("/zb/file-gen/outline/complete", auth, g.CompleteOutlineNode) // 人工标记章节写作完成/取消完成

	// ===== AI 生成（SSE）=====
	e.POST("/zb/file-gen/generate/full", auth, g.GenerateFull)
	e.POST("/zb/file-gen/generate/chapter", auth, g.GenerateChapter)
	e.GET("/zb/file-gen/generate/events", auth, g.SubscribeGenerateEvents)
	e.POST("/zb/file-gen/generate/cancel", auth, g.CancelGenerate)
	e.POST("/zb/file-gen/rewrite", auth, g.Rewrite) // AI 重写选中片段（SSE，无落库）

	// ===== 导出 =====
	e.POST("/zb/file-gen/export/pdf", auth, g.ExportPDF)
	e.POST("/zb/file-gen/export/page-map", auth, g.ExportPageMap) // 两遍导出第一遍：回读章节页码
	e.POST("/zb/file-gen/export/record", auth, g.RecordExport)
}

// registerBidReview 投标书审核模块
func registerBidReview(e gin.IRouter) {
	r := bidreview.GetInstance()
	// ===== 项目管理 =====
	e.POST("/zb/review/project/create", auth, r.CreateProject)              // 上传招/投文件创建（可关联招标解析项目）
	e.POST("/zb/review/project/create-from-gen", auth, r.CreateFromGen)     // 从标书生成项目发起
	e.GET("/zb/review/project/list", auth, r.PageListProject)               // 项目列表
	e.GET("/zb/review/project/detail", auth, r.GetProjectDetail)            // 项目详情
	e.DELETE("/zb/review/project/delete", auth, r.DeleteProject)            // 删除项目
	e.POST("/zb/review/project/retry-stage/:id", auth, r.RetryStage)        // 阶段断点重跑（重置该阶段及后续）
	e.POST("/zb/review/project/cancel/:id", auth, r.CancelProject)          // 取消执行中的审核任务
	e.POST("/zb/review/project/anonymous/:id", auth, r.UpdateAnonymousFlag) // 暗标评审开关
	e.GET("/zb/review/source-pdf", auth, r.GetSourcePdf)                    // 原文 PDF 流

	// ===== 清单项 =====
	e.POST("/zb/review/item/add", auth, r.AddChecklistItem)         // 用户自定义检查项
	e.POST("/zb/review/item/update", auth, r.UpdateChecklistItem)   // 人工确认/驳回 + 备注
	e.POST("/zb/review/item/recheck", auth, r.RecheckChecklistItem) // 单项/整批复检
	e.DELETE("/zb/review/item/delete", auth, r.DeleteChecklistItem) // 删除自定义检查项

	// ===== 整改闭环 =====
	e.POST("/zb/review/remediation/update", auth, r.UpdateRemediation) // 整改状态/责任人

	// ===== 企业规则库 =====
	e.GET("/zb/review/rule/list", auth, r.ListRules)
	e.POST("/zb/review/rule/save", auth, r.SaveRule)
	e.DELETE("/zb/review/rule/delete", auth, r.DeleteRule)
	e.POST("/zb/review/rule/from-item", auth, r.SaveRuleFromItem)

	// ===== 导出 =====
	e.POST("/zb/review/export/report", auth, r.ExportReport) // 导出审核报告（多 sheet Excel）
}
