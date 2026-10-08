package bidreview

import bidreviewRepo "bid-engine/pkg/repo/bidreview"

// ================================================================
// 维度 / 结论 / 状态常量在业务层的短别名（常量真源在 repo 层）
// ================================================================

const (
	DimensionCompliance      = bidreviewRepo.DimensionCompliance
	DimensionCompleteness    = bidreviewRepo.DimensionCompleteness
	DimensionCompetitiveness = bidreviewRepo.DimensionCompetitiveness
	DimensionFormat          = bidreviewRepo.DimensionFormat

	FindingPass     = bidreviewRepo.FindingPass
	FindingWarning  = bidreviewRepo.FindingWarning
	FindingError    = bidreviewRepo.FindingError
	FindingNA       = bidreviewRepo.FindingNA
	FindingNotFound = bidreviewRepo.FindingNotFound

	CheckPending   = bidreviewRepo.CheckPending
	CheckConfirmed = bidreviewRepo.CheckConfirmed
	CheckRejected  = bidreviewRepo.CheckRejected

	RemediationTodo    = bidreviewRepo.RemediationTodo
	RemediationDoing   = bidreviewRepo.RemediationDoing
	RemediationDone    = bidreviewRepo.RemediationDone
	RemediationIgnored = bidreviewRepo.RemediationIgnored

	SourcePreset   = bidreviewRepo.SourcePreset
	SourceAnalysis = bidreviewRepo.SourceAnalysis
	SourceRule     = bidreviewRepo.SourceRule
	SourceUser     = bidreviewRepo.SourceUser
	SourceFormat   = bidreviewRepo.SourceFormat
)

// Dimensions 维度展示顺序
var Dimensions = bidreviewRepo.Dimensions
