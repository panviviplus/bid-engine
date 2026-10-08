package bidanalysisv3

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"bid-engine/pkg/db/model"
	repov3 "bid-engine/pkg/repo/bidanalysisv3"
	repollm "bid-engine/pkg/repo/llm"
)

// CreateInternalProject 为标书生成模块建立不可见的 V3 解析项目。
func (s *Service) CreateInternalProject(ctx context.Context, name, fileName, bucket, object, sha string, userID int64, teamID, companyID int32) (*model.BidAnalysisV3Project, *model.BidAnalysisV3ParseRun, error) {
	llmCtx := repollm.WithUserID(ctx, userID)
	cfg := repollm.ResolveConfig(llmCtx, llmFeatureFactExtract)
	config := map[string]any{"context_window_tokens": 32768, "max_output_tokens": 8192}
	if cfg != nil {
		config["model"] = cfg.Model
		config["endpoint_path"] = cfg.EndpointPath
		config["context_window_tokens"] = cfg.ContextWindowTokens
		config["max_output_tokens"] = cfg.DefaultMaxTokens
	}
	raw, err := json.Marshal(config)
	if err != nil {
		return nil, nil, err
	}
	return s.repo.CreateProjectAndRun(ctx, repov3.CreateProjectInput{Name: name, SourceFileName: fileName, SourceBucket: bucket, SourceObject: object, SourceSHA256: sha, UserID: userID, TeamID: teamID, CompanyID: companyID, Internal: true, ModelConfigJSON: string(raw)})
}

func (s *Service) RunForBidGen(ctx context.Context, projectID, userID int64) error {
	run, err := s.repo.CurrentRun(ctx, projectID)
	if err != nil {
		return err
	}
	if run.Status == repov3.ProjectFailed {
		run, err = s.repo.CreateRun(ctx, projectID, userID, "retry", run.ModelConfigJSON)
		if err != nil {
			return err
		}
	} else if run.Status == repov3.ProjectSucceeded || run.Status == repov3.ProjectSucceededWithWarnings {
		return nil
	}
	return s.RunPipeline(ctx, projectID, run.ID, userID)
}

func (s *Service) BlueprintNodes(ctx context.Context, projectID int64) ([]*model.BidAnalysisV3Blueprint, error) {
	p, err := s.repo.Project(ctx, projectID, 0)
	if err != nil {
		return nil, err
	}
	var nodes []*model.BidAnalysisV3Blueprint
	err = s.repo.DB().WithContext(ctx).Where("project_id=? AND run_id=?", projectID, p.CurrentRunID).Order("sort_order,id").Find(&nodes).Error
	return nodes, err
}

func (s *Service) Clauses(ctx context.Context, projectID int64) ([]*model.BidAnalysisV3Clause, error) {
	p, err := s.repo.Project(ctx, projectID, 0)
	if err != nil {
		return nil, err
	}
	var clauses []*model.BidAnalysisV3Clause
	err = s.repo.DB().WithContext(ctx).Where("project_id=? AND run_id=?", projectID, p.CurrentRunID).Order("chapter_id,sort_order").Find(&clauses).Error
	return clauses, err
}

func (s *Service) GenerationContext(ctx context.Context, projectID int64) (string, error) {
	p, err := s.repo.Project(ctx, projectID, 0)
	if err != nil {
		return "", err
	}
	var summary model.BidAnalysisV3Summary
	if err := s.repo.DB().WithContext(ctx).Where("run_id=?", p.CurrentRunID).First(&summary).Error; err != nil {
		return "", err
	}
	var facts []struct {
		Name     string `json:"name"`
		Value    string `json:"value"`
		Category string `json:"category"`
	}
	if err := s.repo.DB().WithContext(ctx).Table("bid_analysis_v3_field f").Select("f.display_name name,v.display_value value,f.category_key category").Joins("JOIN bid_analysis_v3_field_value v ON v.field_id=f.id").Where("f.project_id=? AND (v.run_id=? OR v.origin='user') AND v.value_status='active'", projectID, p.CurrentRunID).Scan(&facts).Error; err != nil {
		return "", err
	}
	raw, err := json.Marshal(map[string]any{"summary": json.RawMessage(defaultJSON(summary.SummaryJSON)), "facts": facts})
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func (s *Service) DeleteInternalProject(ctx context.Context, projectID, userID int64) error {
	cancelKey := fmt.Sprintf("cancel:tender_parse_v3:%d", projectID)
	if err := s.redis.Client().Set(ctx, cancelKey, "1", 24*time.Hour).Err(); err != nil {
		return fmt.Errorf("无法安全撤销内部解析任务: %w", err)
	}
	jobID, err := s.repo.DeleteProject(ctx, projectID, userID, userID, "关联标书生成项目删除")
	if err != nil {
		if cleanupErr := s.redis.Client().Del(ctx, cancelKey).Err(); cleanupErr != nil {
			s.logger.Warnw("删除内部V3项目失败后回滚取消标记失败", "project_id", projectID, "err", cleanupErr)
		}
		return fmt.Errorf("删除内部解析项目: %w", err)
	}
	go func() {
		if err := s.processDeletionJob(context.Background(), jobID, projectID); err != nil {
			s.logger.Warnw("内部V3项目后台对象清理未完成", "project_id", projectID, "job_id", jobID, "err", err)
		}
	}()
	return nil
}
