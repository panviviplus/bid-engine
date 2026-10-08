package bidreview

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm/clause"

	"bid-engine/pkg/db/model"
	"bid-engine/pkg/repo/taskqueue"
)

func jsonMarshal(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func jsonUnmarshal(raw string, out any) error {
	return json.Unmarshal([]byte(raw), out)
}

// gormExprIncr 生成自增表达式（如 attempts = attempts + 1）
func gormExprIncr(column string, delta int) clause.Expr {
	return clause.Expr{SQL: fmt.Sprintf("%s + ?", column), Vars: []interface{}{delta}}
}

// taskqueueEnqueueOpts 入队选项（项目维度锁与用户归属）
func taskqueueEnqueueOpts(userID, projectID int64) taskqueue.EnqueueOpts {
	return taskqueue.EnqueueOpts{UserID: userID, ProjectID: projectID}
}

// convertAndUploadPdf 非 PDF 投标文件转 PDF 并上传，返回对象键与访问键（失败返回空串，不阻塞解析）
func (s *svcImpl) convertAndUploadPdf(ctx context.Context, projectID int64, f *model.BidReviewV2File) (string, string) {
	_ = os.MkdirAll("./tmp", 0o777)
	ext := strings.ToLower(filepath.Ext(f.FileName))
	localSrc := filepath.Join("./tmp", fmt.Sprintf("bid-review-conv-%d-%d%s", f.ID, time.Now().UnixNano(), ext))
	if err := s.oss.Get(ctx, f.FileObject, localSrc); err != nil {
		s.logger.Warnw("下载待转换文件失败", "file_id", f.ID, "err", err)
		return "", ""
	}
	defer func() { _ = os.Remove(localSrc) }()

	localPdf := filepath.Join("./tmp", fmt.Sprintf("bid-review-conv-%d-%d.pdf", f.ID, time.Now().UnixNano()))
	gc := makeGinCtx(ctx)
	if cerr := s.pdf.Convert2Pdf(gc, localSrc, localPdf); cerr != nil {
		if cerr2 := s.pdf.Convert2PdfBySoffice(gc, localSrc, localPdf); cerr2 != nil {
			s.logger.Warnw("投标文件转PDF失败，使用原文件解析", "file_id", f.ID, "err", cerr2)
			return "", ""
		}
	}
	defer func() { _ = os.Remove(localPdf) }()

	objectKey := fmt.Sprintf("bid-review/%d/pdf/%d.pdf", projectID, f.ID)
	if err := s.oss.Put(ctx, objectKey, localPdf); err != nil {
		s.logger.Warnw("上传转换后的PDF失败", "file_id", f.ID, "err", err)
		return "", ""
	}
	return objectKey, objectKey
}

// parseInt64 字符串转 int64（失败返回 0）
func parseInt64(v string) int64 {
	var n int64
	_, err := fmt.Sscanf(strings.TrimSpace(v), "%d", &n)
	if err != nil {
		return 0
	}
	return n
}
