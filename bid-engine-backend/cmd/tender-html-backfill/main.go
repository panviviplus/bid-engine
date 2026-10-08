package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"bid-engine/lib/common/config"
	"bid-engine/lib/common/logtool"
	"bid-engine/lib/common/storage"
	"bid-engine/pkg/handler/tenderintel"
)

func main() {
	configPath := flag.String("config", "./conf/conf-local.yml", "配置文件路径")
	apply := flag.Bool("apply", false, "执行回填；默认仅预览")
	batchSize := flag.Int("batch-size", 50, "每批读取数量")
	sourceKey := flag.String("source-key", "", "仅处理指定采集源")
	resumeAfterID := flag.Int64("resume-after-id", 0, "从指定公告 ID 之后继续")
	requestInterval := flag.Duration("request-interval", 500*time.Millisecond, "相邻采集请求间隔")
	limit := flag.Int("limit", 0, "本次最多处理数量，0 表示不限")
	refreshExisting := flag.Bool("refresh-existing", false, "重新抓取已有 HTML 的自动采集公告")
	flag.Parse()

	config.MustLoadConfig(*configPath)
	logtool.MustInitLogger()
	storage.MustInitDB()

	result, err := tenderintel.RunHTMLBackfill(context.Background(), tenderintel.HTMLBackfillOptions{
		Apply: *apply, BatchSize: *batchSize, SourceKey: *sourceKey,
		ResumeAfterID:   *resumeAfterID,
		RequestInterval: *requestInterval,
		Limit:           *limit,
		RefreshExisting: *refreshExisting,
		OnProgress: func(item tenderintel.HTMLBackfillProgress) {
			fmt.Printf("notice_id=%d source_key=%q status=%s url=%q error=%q\n", item.NoticeID, item.SourceKey, item.Status, item.URL, item.Error)
		},
	})
	if err != nil {
		log.Fatalf("回填中断: %v", err)
	}
	fmt.Printf("mode=%s candidates=%d updated=%d skipped=%d failed=%d last_id=%d\n",
		map[bool]string{true: "apply", false: "dry-run"}[*apply],
		result.Candidates, result.Updated, result.Skipped, result.Failed, result.LastID)
}
