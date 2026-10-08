package bidgen

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"bid-engine/pkg/db/model"
	"bid-engine/pkg/entity"
	"bid-engine/pkg/repo/converter"
)

// ExportPDF 导出 PDF：接收前端生成的 DOCX → LibreOffice 转 PDF → 返回文件流
func (s *svcImpl) ExportPDF(c *gin.Context) {
	projectID := parseInt64(c.PostForm("projectId"))
	if projectID <= 0 {
		c.JSON(400, gin.H{"code": 400, "msg": "projectId 不能为空"})
		return
	}
	proj, err := s.repo.GetProjectForUser(c, entity.GetUserIDFromCtx(c), projectID)
	if err != nil || proj == nil {
		c.JSON(404, gin.H{"code": 404, "msg": "项目不存在"})
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"code": 400, "msg": "请上传 DOCX 文件"})
		return
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".docx" && ext != ".doc" {
		c.JSON(400, gin.H{"code": 400, "msg": "仅支持 docx/doc 文件转 PDF"})
		return
	}

	_ = os.MkdirAll("./tmp", 0777)
	localDoc := filepath.Join("./tmp", fmt.Sprintf("bidgen-export-%d%s", time.Now().UnixNano(), ext))
	if err := c.SaveUploadedFile(file, localDoc); err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "文件保存失败: " + err.Error()})
		return
	}
	defer os.Remove(localDoc)

	pdfPath, err := converter.GetInstance().ConvertToPDF(entity.ConvertContext(c), localDoc)
	if err != nil {
		s.logger.Warnw("DOCX转PDF失败", "project_id", projectID, "err", err)
		c.JSON(500, gin.H{"code": 500, "msg": "PDF 转换失败，请确认文档转换服务（doc-converter）已启动"})
		return
	}
	defer os.Remove(pdfPath)

	// 记录导出历史
	stat, _ := os.Stat(pdfPath)
	var fileSize int64
	if stat != nil {
		fileSize = stat.Size()
	}
	baseName := strings.TrimSuffix(file.Filename, filepath.Ext(file.Filename))
	exportName := proj.Name + "-" + baseName + ".pdf"
	_ = s.repo.AddExportRecord(c, &model.BidGenExportRecord{
		ProjectID:  projectID,
		ExportType: "pdf",
		FileName:   exportName,
		FileSize:   fileSize,
		Status:     "succeeded",
		UserID:     entity.GetUserIDFromCtx(c),
	})

	c.Header("Content-Disposition", "attachment; filename=\""+strconv.Quote(exportName)+"\"")
	c.Header("Content-Type", "application/pdf")
	c.File(pdfPath)
}
