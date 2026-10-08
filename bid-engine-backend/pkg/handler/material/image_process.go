package material

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	skbcfg "bid-engine/pkg/config"
	"bid-engine/pkg/db/model"
	"bid-engine/pkg/entity"
	repoAgent "bid-engine/pkg/repo/agent"
	"bid-engine/pkg/utils"
)

const (
	materialImageNameAgentID = "material-image-name"
	materialImageNameQuery   = "请帮我提炼这张图片的名称和描述。"
	materialImageNameUser    = "abc-123"
)

type materialImageNameAnswer struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (s *svcImpl) buildMaterialImageAsyncContext(c *gin.Context) (*gin.Context, context.Context, context.CancelFunc, error) {
	if c == nil {
		return nil, nil, nil, fmt.Errorf("context is nil")
	}
	timeoutStr := strings.TrimSpace(skbcfg.Get("properties.material_image_analysis"))
	timeoutHours := 1
	if timeoutStr != "" {
		if v, err := strconv.Atoi(timeoutStr); err == nil && v > 0 {
			timeoutHours = v
		}
	}
	flowCtx, flowCancel := context.WithTimeout(context.Background(), time.Duration(timeoutHours)*time.Hour)

	ac := c.Copy()
	rid := entity.GetRequestIDForGo(c)
	if rid != "" {
		entity.SetRequestID(ac, rid)
	}
	if c.Request != nil {
		ac.Request = c.Request.Clone(flowCtx)
	} else {
		req, _ := http.NewRequestWithContext(flowCtx, "GET", "/", nil)
		ac.Request = req
	}

	return ac, flowCtx, flowCancel, nil
}

// asyncFillTargetImageNameDescByImageID 异步填充指定图片的名称和描述
func (s *svcImpl) asyncFillTargetImageNameDescByImageID(c *gin.Context, imageId int64) {
	if imageId <= 0 {
		return
	}
	ac, flowCtx, flowCancel, err := s.buildMaterialImageAsyncContext(c)
	if err != nil {
		return
	}
	defer flowCancel()
	_ = s.processOneImageNameDesc(ac, flowCtx, imageId)
}

// asyncFillImageNameDescByMaterialID 异步填充素材图片的名称和描述
func (s *svcImpl) asyncFillImageNameDescByMaterialID(c *gin.Context, materialID int64) {
	if materialID <= 0 {
		return
	}
	ac, flowCtx, flowCancel, err := s.buildMaterialImageAsyncContext(c)
	if err != nil {
		return
	}
	defer flowCancel()

	imgs, err := s.repo.ListMaterialImages(ac, materialID)
	if err != nil || len(imgs) == 0 {
		return
	}
	s.fillImageNameDescForImages(ac, flowCtx, imgs)
}

// asyncFillImageNameDescForUserGallery 异步填充用户素材库图片的名称和描述
func (s *svcImpl) asyncFillImageNameDescForUserGallery(c *gin.Context, userID int64) {
	if userID <= 0 {
		return
	}
	ac, flowCtx, flowCancel, err := s.buildMaterialImageAsyncContext(c)
	if err != nil {
		return
	}
	defer flowCancel()

	imgs, err := s.repo.ListMaterialImagesByUserID(ac, userID)
	if err != nil || len(imgs) == 0 {
		return
	}
	filtered := make([]*model.MaterialImageInfo, 0, len(imgs))
	for _, im := range imgs {
		if im == nil {
			continue
		}
		if im.MaterialID == 0 {
			filtered = append(filtered, im)
		}
	}
	s.fillImageNameDescForImages(ac, flowCtx, filtered)
}

func (s *svcImpl) fillImageNameDescForImages(ac *gin.Context, flowCtx context.Context, images []*model.MaterialImageInfo) {
	for _, im := range images {
		if im == nil {
			continue
		}
		if strings.TrimSpace(im.Name) != "" && strings.TrimSpace(im.Description) != "" {
			continue
		}
		_ = s.processOneImageNameDesc(ac, flowCtx, im.ID)
	}
}

func (s *svcImpl) processOneImageNameDesc(ac *gin.Context, flowCtx context.Context, imageID int64) error {
	if imageID <= 0 {
		return fmt.Errorf("invalid imageID")
	}

	_ = os.MkdirAll("./logs/material", 0o777)
	logFile := filepath.Join("./logs/material", fmt.Sprintf("%d-%s.log", imageID, time.Now().Format("20060102-150405")))
	mf, _ := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if mf != nil {
		defer func() { _ = mf.Close() }()
	}
	writeLog := func(msg string, kv ...interface{}) {
		if mf != nil {
			ts := time.Now().Format(time.RFC3339)
			_, _ = fmt.Fprintln(mf, ts, msg, fmt.Sprint(kv...))
		}
	}

	writeLog("准备调用智能体，提炼图片名称和描述。", "imageID", imageID)

	var img *model.MaterialImageInfo
	var err error
	img, err = s.repo.GetMaterialImage(ac, imageID)
	if err != nil || img == nil {
		writeLog(">>>>>> 查询material_image_info表记录失败!!!", "err", err)
		if err != nil {
			return err
		}
		return fmt.Errorf("image not found")
	}

	needName := strings.TrimSpace(img.Name) == ""
	needDesc := strings.TrimSpace(img.Description) == ""
	if !needName && !needDesc {
		writeLog("跳过！", "理由：", "名称和描述均已填充！")
		return nil
	}

	objectKey := strings.TrimSpace(img.ObjectKey)
	if objectKey == "" {
		objectKey = strings.TrimSpace(img.OriginObjectKey)
	}
	if objectKey == "" {
		writeLog("跳过！", "理由：", "图片object_key为空！")
		return fmt.Errorf("object key empty")
	}

	ext := filepath.Ext(objectKey)
	if strings.TrimSpace(ext) == "" {
		ext = ".png"
	}
	_ = os.MkdirAll("./tmp", 0o777)
	localPath := filepath.Join("./tmp", fmt.Sprintf("material-image-%d-%d%s", imageID, time.Now().UnixNano(), ext))
	ossCtx := entity.ConvertContext(ac)
	if dl, ok := flowCtx.Deadline(); ok {
		ctx2, cancel2 := context.WithDeadline(ossCtx, dl)
		defer cancel2()
		ossCtx = ctx2
	}
	if getErr := s.oss.Get(ossCtx, objectKey, localPath); getErr != nil {
		writeLog(">>>>>> 从OSS下载图片失败!!!", "objectKey", objectKey, "err", getErr)
		return getErr
	}
	defer func() { _ = os.Remove(localPath) }()

	cfg, cfgErr := s.agentModel.GetAgentConfig(materialImageNameAgentID)
	if cfgErr != nil || cfg == nil || strings.TrimSpace(cfg.Token) == "" {
		writeLog(">>>>>> 读取“图片取名”智能体配置失败!!!", "err", cfgErr)
		if cfgErr != nil {
			return cfgErr
		}
		return fmt.Errorf("agent config missing token")
	}

	uploadID, err := utils.UploadFileToDify(cfg.Token, localPath)
	if err != nil {
		writeLog(">>>>>> 给智能体上传图片失败!!!", "err", err)
		return err
	}

	agentReq := &repoAgent.AgentRequest{
		ResponseMode:   "blocking",
		ConversationID: "",
		Query:          materialImageNameQuery,
		Inputs:         map[string]interface{}{},
		User:           materialImageNameUser,
		Files: []interface{}{
			map[string]interface{}{
				"type":            "image",
				"transfer_method": "local_file",
				"upload_file_id":  uploadID,
			},
		},
	}
	writeLog("\n\n================智能体请求参数================\n\n", "req", agentReq)

	resp, err := s.agentModel.CallAgent(flowCtx, materialImageNameAgentID, agentReq)
	if err != nil {
		writeLog(">>>>>> 调用智能体失败!!!", "err", err)
		return err
	}
	writeLog("\n\n================智能体返回值================\n\n", "resp", resp)

	raw := strings.TrimSpace(resp.Answer)
	var ans materialImageNameAnswer
	if raw != "" {
		if unmarshalErr := json.Unmarshal([]byte(raw), &ans); unmarshalErr != nil {
			writeLog(">>>>>> 解析返回值Answer字段失败!!!", "err", unmarshalErr, "raw", raw)
			return unmarshalErr
		}
	}

	updateFields := map[string]interface{}{}
	if needName {
		if v := strings.TrimSpace(ans.Name); v != "" {
			name := v
			if strings.TrimSpace(ext) != "" {
				ln := strings.ToLower(strings.TrimSpace(name))
				le := strings.ToLower(strings.TrimSpace(ext))
				if !strings.HasSuffix(ln, le) {
					name = name + ext
				}
			}
			updateFields["name"] = name
		}
	}
	if needDesc {
		if v := strings.TrimSpace(ans.Description); v != "" {
			updateFields["description"] = v
		}
	}
	if len(updateFields) == 0 {
		writeLog(">>>>>> 跳过更新！", "理由：", "智能体返回值中，名称和描述均为空！")
		return nil
	}

	var latest *model.MaterialImageInfo
	var gerr error
	latest, gerr = s.repo.GetMaterialImage(ac, imageID)
	if gerr != nil || latest == nil {
		writeLog(">>>>>> 从数据库加载图片信息失败!!!", "err", gerr)
		if gerr != nil {
			return gerr
		}
		return fmt.Errorf("image not found")
	}
	if strings.TrimSpace(latest.Name) != "" {
		delete(updateFields, "name")
	}
	if strings.TrimSpace(latest.Description) != "" {
		delete(updateFields, "description")
	}
	if len(updateFields) == 0 {
		writeLog("跳过更新！", "理由：", "更新前发现：图片名称和描述均已填充！")
		return nil
	}

	if err := s.repo.UpdateMaterialImageInfo(ac, imageID, updateFields); err != nil {
		writeLog(">>>>>> 更新数据库图片信息失败!!!", "err", err)
		return err
	}
	writeLog("喜大普奔！更新数据库图片信息成功！", updateFields)

	return nil
}
