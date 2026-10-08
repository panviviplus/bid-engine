package bidanalysisv3

import (
	"context"
	"time"
)

// controlStaleTimeout 控制请求从发起到应用的容忍时长。
// 流水线存活时运行控制观察器每 300ms 轮询一次，1 秒内即可应用；
// 超过该时长仍未被应用，说明解析任务已中断（孤儿任务/未运行），应显式失败给用户反馈。
const controlStaleTimeout = 3 * time.Minute

// StartControlSweeper 定期把长时间未应用的暂停/跳过控制标记为失败，
// 避免解析任务中断时前端无限等待“正在暂停/正在跳过”。
func (s *Service) StartControlSweeper(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	s.sweepStaleControls(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sweepStaleControls(ctx)
		}
	}
}

func (s *Service) sweepStaleControls(ctx context.Context) {
	affected, err := s.repo.FailStaleControls(ctx, time.Now().Add(-controlStaleTimeout))
	if err != nil {
		s.logger.Warnw("清扫过期运行控制失败", "err", err)
		return
	}
	if affected > 0 {
		s.logger.Infow("过期运行控制已标记失败", "count", affected)
	}
}
