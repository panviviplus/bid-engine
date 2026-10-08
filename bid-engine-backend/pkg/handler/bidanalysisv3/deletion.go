package bidanalysisv3

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"bid-engine/pkg/repo/taskqueue"
	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func (s *Service) StartDeletionWorker(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	s.processPendingDeletions(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.processPendingDeletions(ctx)
		}
	}
}

func (s *Service) HandleTaskFinal(ctx context.Context, task *taskqueue.Task) {
	if task == nil {
		return
	}
	if _, err := s.repo.Project(ctx, task.ProjectID, 0); errors.Is(err, gorm.ErrRecordNotFound) {
		if cleanupErr := s.cleanupProjectTasks(ctx, task.ProjectID); cleanupErr != nil {
			s.logger.Warnw("清理已删除项目的 Redis 任务失败", "project_id", task.ProjectID, "err", cleanupErr)
		}
	}
}

func (s *Service) processPendingDeletions(ctx context.Context) {
	jobs, err := s.repo.PendingDeletionJobs(ctx, 10)
	if err != nil {
		s.logger.Errorw("读取V3删除任务失败", "err", err)
		return
	}
	for _, job := range jobs {
		if err := s.processDeletionJob(ctx, job.ID, job.OriginalProjectID); err != nil {
			s.logger.Warnw("V3对象清理未完成", "job_id", job.ID, "err", err)
		}
	}
}

func (s *Service) processDeletionJob(ctx context.Context, jobID, projectID int64) error {
	claimed, err := s.repo.ClaimDeletionJob(ctx, jobID)
	if err != nil {
		return err
	}
	if !claimed {
		job, err := s.repo.DeletionJob(ctx, jobID)
		if err != nil {
			return err
		}
		// An idempotent DELETE recreates the cancellation marker before it
		// discovers the completed tombstone. Remove that marker only after the
		// existing cleanup job is known to be fully complete.
		if job.Status == "succeeded" {
			return s.cleanupProjectTasks(ctx, projectID)
		}
		return nil
	}
	objects, err := s.repo.DeletionObjects(ctx, jobID)
	if err != nil {
		return err
	}
	var firstErr error
	for _, object := range objects {
		err := s.oss.Delete(ctx, object.ObjectKey)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			if markErr := s.repo.MarkDeletionObject(ctx, object.ID, false, err.Error()); markErr != nil && firstErr == nil {
				firstErr = markErr
			}
			continue
		}
		if markErr := s.repo.MarkDeletionObject(ctx, object.ID, true, ""); markErr != nil && firstErr == nil {
			firstErr = markErr
		}
	}
	if err := s.cleanupProjectTasks(ctx, projectID); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := s.repo.FinishDeletionJob(ctx, jobID); err != nil {
		return err
	}
	return firstErr
}

func (s *Service) cleanupProjectTasks(ctx context.Context, projectID int64) error {
	for _, taskType := range []string{queueType, blueprintQueueType} {
		cfg := taskqueue.QueueConfigs[taskType]
		var cursor uint64
		for {
			keys, next, err := s.redis.Client().Scan(ctx, cursor, "task:"+taskType+"_*", 100).Result()
			if err != nil {
				return err
			}
			for _, key := range keys {
				value, err := s.redis.Client().HGet(ctx, key, "project_id").Result()
				if err != nil {
					if errors.Is(err, goredis.Nil) {
						continue
					}
					return err
				}
				id, err := strconv.ParseInt(value, 10, 64)
				if err != nil {
					return fmt.Errorf("任务 %s 的 project_id 无效: %w", key, err)
				}
				if id != projectID {
					continue
				}
				taskID := strings.TrimPrefix(key, "task:")
				pipe := s.redis.Client().TxPipeline()
				pipe.LRem(ctx, cfg.QueueKey, 0, taskID)
				pipe.LRem(ctx, cfg.HighQueueKey, 0, taskID)
				pipe.LRem(ctx, cfg.DLQKey, 0, taskID)
				pipe.ZRem(ctx, taskqueue.DelayedQueueKey, taskType+"|"+taskID)
				pipe.Del(ctx, key)
				if _, err := pipe.Exec(ctx); err != nil {
					return err
				}
			}
			cursor = next
			if cursor == 0 {
				break
			}
		}
	}
	keys := []string{fmt.Sprintf("active:tender_parse_v3:%d", projectID), fmt.Sprintf("lock:tender_parse_v3:%d", projectID), fmt.Sprintf("cancel:tender_parse_v3:%d", projectID)}
	return s.redis.Client().Del(ctx, keys...).Err()
}
