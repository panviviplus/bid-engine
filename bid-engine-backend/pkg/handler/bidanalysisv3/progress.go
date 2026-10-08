package bidanalysisv3

import (
	"context"
	"time"
)

func runStageProgressReporter(ctx context.Context, events <-chan bool, flushEvery int, interval time.Duration, update func(completed, failed int) error) error {
	if flushEvery <= 0 {
		flushEvery = 5
	}
	if interval <= 0 {
		interval = 2 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	completed, failed, pending := 0, 0, 0
	flush := func() error {
		if pending == 0 {
			return nil
		}
		if err := update(completed, failed); err != nil {
			return err
		}
		pending = 0
		return nil
	}
	for {
		select {
		case <-ctx.Done():
			if err := flush(); err != nil {
				return err
			}
			return ctx.Err()
		case success, ok := <-events:
			if !ok {
				return flush()
			}
			if success {
				completed++
			} else {
				failed++
			}
			pending++
			if pending >= flushEvery {
				if err := flush(); err != nil {
					return err
				}
			}
		case <-ticker.C:
			if err := flush(); err != nil {
				return err
			}
		}
	}
}
