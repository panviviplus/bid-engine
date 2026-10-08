package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"strings"

	"gorm.io/gorm"

	"bid-engine/lib/common/config"
	"bid-engine/lib/common/logtool"
	"bid-engine/lib/common/storage"
	"bid-engine/pkg/db/model"
	"bid-engine/pkg/handler/bidgen"
	"bid-engine/pkg/service/biddoc"
)

func main() {
	configPath := flag.String("config", "./conf/conf-local.yml", "配置文件路径")
	apply := flag.Bool("apply", false, "执行修复；默认仅预览")
	flag.Parse()

	config.MustLoadConfig(*configPath)
	logtool.MustInitLogger()
	storage.MustInitDB()
	db := storage.GetDB()
	ctx := context.Background()

	var projects []*model.BidGenProject
	if err := db.WithContext(ctx).Order("id").Find(&projects).Error; err != nil {
		log.Fatalf("加载标书项目失败: %v", err)
	}
	candidates, repaired, normalized, skipped, failed := 0, 0, 0, 0, 0
	for _, project := range projects {
		var outlines []*model.BidGenOutline
		if err := db.WithContext(ctx).Where("project_id=?", project.ID).Find(&outlines).Error; err != nil {
			fmt.Printf("project=%d status=error reason=%q\n", project.ID, err.Error())
			failed++
			continue
		}
		if len(outlines) == 0 {
			continue
		}
		completed, total := 0, 0
		for _, outline := range outlines {
			if biddoc.IsDocumentRoot(outline) {
				continue
			}
			total++
			if outline.GenStatus == "succeeded" {
				completed++
			}
		}
		overallProgress := 0
		if total > 0 {
			overallProgress = completed * 100 / total
		}
		if project.Status == "draft" && (project.Stage != "" || project.Progress != int32(overallProgress) || project.LastError != "") {
			if !*apply {
				fmt.Printf("project=%d status=would_normalize_state progress=%d\n", project.ID, overallProgress)
			} else if err := db.WithContext(ctx).Model(project).Updates(map[string]interface{}{
				"stage": "", "progress": overallProgress, "last_error": "",
			}).Error; err != nil {
				fmt.Printf("project=%d status=error reason=%q\n", project.ID, err.Error())
				failed++
				continue
			} else {
				fmt.Printf("project=%d status=normalized_state progress=%d\n", project.ID, overallProgress)
				normalized++
			}
		}
		var doc model.BidGenDocContent
		err := db.WithContext(ctx).Where("project_id=?", project.ID).First(&doc).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			fmt.Printf("project=%d status=error reason=%q\n", project.ID, err.Error())
			failed++
			continue
		}
		state := biddoc.AnchorEmpty
		missing := len(outlines)
		if err == nil && strings.TrimSpace(doc.DocJSON) != "" {
			inspection, inspectErr := biddoc.InspectOutlineAnchors(doc.DocJSON, outlines)
			if inspectErr != nil {
				fmt.Printf("project=%d status=error reason=%q\n", project.ID, inspectErr.Error())
				failed++
				continue
			}
			state = inspection.State
			missing = len(inspection.Missing)
		}
		if state == biddoc.AnchorComplete {
			continue
		}
		if state == biddoc.AnchorPartial {
			fmt.Printf("project=%d status=skipped reason=%q missing=%d\n", project.ID, "正文仅部分包含大纲锚点", missing)
			skipped++
			continue
		}
		candidates++
		if !*apply {
			fmt.Printf("project=%d status=would_repair outlines=%d\n", project.ID, len(outlines))
			continue
		}
		if err := bidgen.RepairProjectDocument(ctx, project.ID); err != nil {
			fmt.Printf("project=%d status=error reason=%q\n", project.ID, err.Error())
			failed++
			continue
		}
		fmt.Printf("project=%d status=repaired outlines=%d\n", project.ID, len(outlines))
		repaired++
	}
	fmt.Printf("mode=%s candidates=%d repaired=%d normalized=%d skipped=%d failed=%d\n", map[bool]string{true: "apply", false: "dry-run"}[*apply], candidates, repaired, normalized, skipped, failed)
	if failed > 0 {
		log.Fatalf("存在 %d 个修复失败项目", failed)
	}
}
