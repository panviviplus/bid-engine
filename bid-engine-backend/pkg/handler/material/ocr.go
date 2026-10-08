package material

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"bid-engine/pkg/db/model"
	"bid-engine/pkg/entity"
	"bid-engine/pkg/handler"
	"bid-engine/pkg/repo/docling"
	repoLLM "bid-engine/pkg/repo/llm"
	repoMaterial "bid-engine/pkg/repo/material"
)

const llmFeatOcrExtract = "material_ocr_extract"

// asyncRunOcr 异步 OCR 解析素材的所有文件
func (s *svcImpl) asyncRunOcr(c *gin.Context, materialID int64) {
	log := s.logger.With(entity.Ctx(c)...)
	summary, err := s.repo.LookupMaterialForUser(c.Request.Context(), entity.GetUserIDFromCtx(c), materialID)
	if err != nil || summary == nil {
		log.Warnw("OCR: 素材不存在", "material_id", materialID, "err", err)
		return
	}
	// 模板素材不做 OCR
	if summary.Type == "template" {
		return
	}

	files, err := s.repo.ListMaterialFiles(c, materialID)
	if err != nil || len(files) == 0 {
		return
	}

	allOK := true
	for _, f := range files {
		if f == nil {
			continue
		}
		if err := s.processOneFileOcr(c, materialID, f); err != nil {
			log.Warnw("OCR: 单文件处理失败", "file_id", f.ID, "name", f.Name, "err", err)
			allOK = false
		}
	}
	_ = allOK
}

func (s *svcImpl) processOneFileOcr(c *gin.Context, materialID int64, f *model.MaterialFileInfo) error {
	log := s.logger.With(entity.Ctx(c)...)

	// 1. 创建 pending 记录
	rec := &model.MaterialOcrResult{
		MaterialID: materialID,
		FileID:     f.ID,
		Status:     "pending",
	}
	if err := s.repo.CreateOcrResult(c, rec); err != nil {
		return fmt.Errorf("创建 OCR 记录失败: %w", err)
	}

	// 2. 下载文件
	if err := os.MkdirAll("./tmp", 0777); err != nil {
		return fmt.Errorf("创建临时目录失败: %w", err)
	}
	ext := strings.ToLower(filepath.Ext(f.Name))
	localFile := fmt.Sprintf("./tmp/ocr-%d-%d%s", materialID, f.ID, ext)
	defer os.Remove(localFile)

	if err := s.oss.Get(c, f.ObjectKey, localFile); err != nil {
		_ = s.repo.UpdateOcrResult(c, rec.ID, map[string]interface{}{
			"status": "failed", "error_message": fmt.Sprintf("下载文件失败: %v", err),
		})
		return fmt.Errorf("下载文件失败: %w", err)
	}

	// 3. 更新状态为 processing
	_ = s.repo.UpdateOcrResult(c, rec.ID, map[string]interface{}{"status": "processing"})

	// 4. Docling 解析（启用 OCR）
	ocrOpts := s.buildDoclingOcrOpts(f)
	result, err := s.docling.Parse(c.Request.Context(), localFile, ocrOpts)
	if err != nil || result == nil {
		errMsg := fmt.Sprintf("Docling 解析失败: %v", err)
		_ = s.repo.UpdateOcrResult(c, rec.ID, map[string]interface{}{
			"status": "failed", "error_message": errMsg,
		})
		return errors.New(errMsg)
	}

	// 5. 存储原始解析结果
	rawJSON, _ := json.Marshal(result.JSON)
	updates := map[string]interface{}{
		"ocr_raw_text": strings.TrimSpace(result.Text),
		"ocr_raw_json": string(rawJSON),
	}
	_ = s.repo.UpdateOcrResult(c, rec.ID, updates)

	// 5.5 仅认用户 LLM 配置：提取前先做存在性检查，未配置则写入原因并跳过
	llmCtx := repoLLM.WithUserID(c.Request.Context(), entity.GetUserIDFromCtx(c))
	if err := s.llm.LlmConfigExist(llmCtx, repoLLM.ModuleMaterial); err != nil {
		// 落库只写分类后的友好文案，原始错误细节进服务端日志
		_, msg := repoLLM.LLMErrorMeta(err)
		_ = s.repo.UpdateOcrResult(c, rec.ID, map[string]interface{}{
			"status":        "done",
			"error_message": msg,
		})
		log.Warnw("OCR: 跳过 LLM 提取（LLM 未配置）", "ocr_id", rec.ID, "err", err)
		return nil
	}

	// 6. LLM 结构化提取
	if result.Text != "" {
		fields, labels, err := s.extractStructuredFields(c, rec.ID, infoFetcher{repo: s.repo, c: c, materialID: materialID})
		if err != nil {
			log.Warnw("OCR: LLM 提取失败", "ocr_id", rec.ID, "err", err)
		}
		sfJSON, _ := json.Marshal(fields)
		_ = s.repo.UpdateOcrResult(c, rec.ID, map[string]interface{}{
			"structured_fields": string(sfJSON),
			"labels":            strings.Join(labels, ","),
			"status":            "done",
		})
	} else {
		_ = s.repo.UpdateOcrResult(c, rec.ID, map[string]interface{}{"status": "done"})
	}

	return nil
}

type infoFetcher struct {
	repo       repoMaterial.Service // this is the handler's repo
	c          *gin.Context
	materialID int64
}

func (f infoFetcher) GetType() string {
	summary, err := f.repo.LookupMaterialForUser(f.c.Request.Context(), entity.GetUserIDFromCtx(f.c), f.materialID)
	if err != nil || summary == nil {
		return "qualification"
	}
	return summary.Type
}

func (f infoFetcher) GetDescription() string {
	summary, err := f.repo.LookupMaterialForUser(f.c.Request.Context(), entity.GetUserIDFromCtx(f.c), f.materialID)
	if err != nil || summary == nil {
		return ""
	}
	return summary.Description
}

func (s *svcImpl) buildDoclingOcrOpts(_ *model.MaterialFileInfo) *docling.ParseOptions {
	doOCR := true
	return &docling.ParseOptions{
		DoOCR:   &doOCR,
		OCRLang: []string{"zh"},
	}
}

func (s *svcImpl) extractStructuredFields(c *gin.Context, ocrID int64, info infoFetcher) (map[string]interface{}, []string, error) {
	rec, err := s.repo.GetOcrResultByID(c, ocrID)
	if err != nil || rec == nil {
		return nil, nil, fmt.Errorf("OCR 记录不存在")
	}
	if rec.OcrRawText == "" {
		return map[string]interface{}{}, []string{}, nil
	}

	mt := info.GetType()
	desc := info.GetDescription()
	text := rec.OcrRawText
	if len([]rune(text)) > 15000 {
		text = string([]rune(text)[:15000])
	}

	var sp, schemaHint string
	if mt == "qualification" {
		sp = "你是一个企业资质信息提取专家。从OCR识别文本中提取资质证书的结构化信息。"
		schemaHint = `{cert_type:"", cert_number:"", holder_name:"", issuing_authority:"", valid_from:"", valid_to:"", scope:""}, 以及 labels: ["关键词1","关键词2"]`
	} else {
		sp = "你是一个企业业绩信息提取专家。从OCR识别文本中提取项目业绩的结构化信息。"
		schemaHint = `{project_name:"", client:"", contract_amount:"", completion_date:"", summary:""}, 以及 labels: ["关键词1","关键词2"]`
	}

	descHint := ""
	if desc != "" {
		descHint = fmt.Sprintf("\n用户提供的描述: %s\n", desc)
	}

	prompt := fmt.Sprintf("请从以下文本中提取%s结构化字段。\n%s\n文本内容:\n%s\n\n输出格式: JSON，包含 fields: {%s}", mt, descHint, text, schemaHint)
	temp := 0.1
	req := &repoLLM.ChatRequest{System: sp, Prompt: prompt, Temperature: &temp}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()
	ctx = repoLLM.WithUserID(ctx, entity.GetUserIDFromCtx(c))

	res, err := s.llm.ChatOnceByFeature(ctx, llmFeatOcrExtract, req)
	if err != nil {
		return nil, nil, err
	}
	if res == nil || strings.TrimSpace(res.Content) == "" {
		return map[string]interface{}{}, []string{}, nil
	}

	var parsed struct {
		Fields map[string]interface{} `json:"fields"`
		Labels []string               `json:"labels"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(res.Content)), &parsed); err != nil {
		// 尝试直接用 LLM 返回的 JSON
		return map[string]interface{}{"raw": res.Content}, []string{}, nil
	}
	return parsed.Fields, parsed.Labels, nil
}

// GetOcrResults 获取素材的所有 OCR 解析结果
func (s *svcImpl) GetOcrResults(c *gin.Context) {
	materialID, err := strconv.ParseInt(c.Query("material_id"), 10, 64)
	if err != nil || materialID <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeParam, "material_id 不能为空", nil)
		return
	}
	if _, err := s.repo.LookupMaterialForUser(c.Request.Context(), entity.GetUserIDFromCtx(c), materialID); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "素材不存在", nil)
		return
	}
	results, err := s.repo.GetOcrResultByMaterialID(c, materialID)
	if err != nil {
		handler.SendNormalResp(c, entity.ErrCodeDBRead, "查询失败", nil)
		return
	}
	if results == nil {
		results = []*model.MaterialOcrResult{}
	}
	handler.SendOKResp(c, results)
}

// RetryOcr 手动重试 OCR
func (s *svcImpl) RetryOcr(c *gin.Context) {
	materialID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || materialID <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeParam, "id 不能为空", nil)
		return
	}
	if _, err := s.repo.LookupMaterialForUser(c.Request.Context(), entity.GetUserIDFromCtx(c), materialID); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "素材不存在", nil)
		return
	}
	// 清除旧数据，重新开始
	if err := s.repo.DeleteOcrResultsByMaterialID(c, materialID); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, "清理旧OCR结果失败", nil)
		return
	}
	go s.asyncRunOcr(c.Copy(), materialID)
	handler.SendOKResp(c, map[string]interface{}{"msg": "OCR 已重新开始"})
}
