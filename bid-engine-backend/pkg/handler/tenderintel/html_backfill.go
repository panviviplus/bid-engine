package tenderintel

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"bid-engine/pkg/db/model"
	intelRepo "bid-engine/pkg/repo/tenderintel"
)

type HTMLBackfillOptions struct {
	Apply           bool
	BatchSize       int
	SourceKey       string
	ResumeAfterID   int64
	RequestInterval time.Duration
	Limit           int
	RefreshExisting bool
	OnProgress      func(HTMLBackfillProgress)
}

type HTMLBackfillResult struct {
	Candidates int
	Updated    int
	Skipped    int
	Failed     int
	LastID     int64
}

type HTMLBackfillProgress struct {
	NoticeID  int64
	SourceKey string
	URL       string
	Status    string
	Error     string
}

type htmlBackfillStore interface {
	ListNoticeHTMLBackfillBatch(ctx context.Context, afterID int64, sourceKey string, limit int, includeExisting bool) ([]*model.TenderIntelNotice, error)
	GetSource(ctx context.Context, sourceKey string) (*model.TenderIntelSource, error)
	RefreshNoticeContent(ctx context.Context, noticeID int64, refresh intelRepo.NoticeContentRefresh) (bool, error)
}

type htmlBackfillExtractor interface {
	Extract(ctx context.Context, req collectorExtractRequest) (*collectorDoc, error)
}

type htmlBackfillRunner struct {
	store     htmlBackfillStore
	extractor htmlBackfillExtractor
	wait      func(context.Context, time.Duration) error
}

func RunHTMLBackfill(ctx context.Context, options HTMLBackfillOptions) (HTMLBackfillResult, error) {
	runner := htmlBackfillRunner{
		store:     intelRepo.New(),
		extractor: newCollectorClient(),
	}
	return runner.Run(ctx, options)
}

func (r *htmlBackfillRunner) Run(ctx context.Context, options HTMLBackfillOptions) (HTMLBackfillResult, error) {
	result := HTMLBackfillResult{LastID: options.ResumeAfterID}
	batchSize := options.BatchSize
	if batchSize <= 0 || batchSize > 500 {
		batchSize = 50
	}
	wait := r.wait
	if wait == nil {
		wait = waitForBackfill
	}
	sources := make(map[string]*model.TenderIntelSource)
	requestCount := 0

	for options.Limit <= 0 || result.Candidates < options.Limit {
		fetchLimit := batchSize
		if options.Limit > 0 && options.Limit-result.Candidates < fetchLimit {
			fetchLimit = options.Limit - result.Candidates
		}
		notices, err := r.store.ListNoticeHTMLBackfillBatch(ctx, result.LastID, options.SourceKey, fetchLimit, options.RefreshExisting)
		if err != nil {
			return result, err
		}
		if len(notices) == 0 {
			break
		}
		for _, notice := range notices {
			result.Candidates++
			result.LastID = notice.ID
			if !options.Apply {
				emitHTMLBackfillProgress(options, notice, "candidate", nil)
				continue
			}
			if requestCount > 0 && options.RequestInterval > 0 {
				if err := wait(ctx, options.RequestInterval); err != nil {
					return result, err
				}
			}
			requestCount++

			source := sources[notice.SourceKey]
			if source == nil {
				source, err = r.store.GetSource(ctx, notice.SourceKey)
				if err != nil {
					result.Failed++
					emitHTMLBackfillProgress(options, notice, "failed", err)
					continue
				}
				sources[notice.SourceKey] = source
			}
			doc, err := r.extractor.Extract(ctx, collectorExtractRequest{
				URL: notice.URL, SourceKey: notice.SourceKey,
				NeedsBrowser: source.NeedsBrowser == 1, IncludeRawHTML: false,
			})
			if err != nil {
				result.Failed++
				emitHTMLBackfillProgress(options, notice, "failed", err)
				continue
			}
			if doc == nil || strings.TrimSpace(doc.BodyHTML) == "" {
				result.Skipped++
				emitHTMLBackfillProgress(options, notice, "skipped", nil)
				continue
			}
			bodyText := firstNonEmpty(doc.BodyText, notice.BodyText)
			bodyMarkdown := firstNonEmpty(doc.BodyMarkdown, notice.BodyMarkdown, bodyText)
			warnings, _ := json.Marshal(doc.Extract.Warnings)
			_, err = r.store.RefreshNoticeContent(ctx, notice.ID, intelRepo.NoticeContentRefresh{
				BodyHTML: strings.TrimSpace(doc.BodyHTML), BodyMarkdown: bodyMarkdown, BodyText: bodyText,
				ContentHash: HashContent(bodyText), FetchStrategy: doc.Extract.Strategy,
				ExtractConfidence: doc.Extract.Confidence, ExtractWarnings: string(warnings), SeenAt: NowFunc(),
			})
			if err != nil {
				result.Failed++
				emitHTMLBackfillProgress(options, notice, "failed", err)
				continue
			}
			result.Updated++
			emitHTMLBackfillProgress(options, notice, "updated", nil)
		}
		if len(notices) < fetchLimit {
			break
		}
	}
	return result, nil
}

func emitHTMLBackfillProgress(options HTMLBackfillOptions, notice *model.TenderIntelNotice, status string, err error) {
	if options.OnProgress == nil {
		return
	}
	progress := HTMLBackfillProgress{
		NoticeID: notice.ID, SourceKey: notice.SourceKey, URL: notice.URL, Status: status,
	}
	if err != nil {
		progress.Error = err.Error()
	}
	options.OnProgress(progress)
}

func waitForBackfill(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
