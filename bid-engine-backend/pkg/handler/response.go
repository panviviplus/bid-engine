package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"bid-engine/pkg/repo/oauth"

	"github.com/gin-contrib/requestid"
	"github.com/gin-contrib/sse"
	"github.com/gin-gonic/gin"

	"bid-engine/pkg/entity"
)

// SendPageRespV2 分页结果返回
func SendPageRespV2(c *gin.Context, data interface{}, total int64, pageNum, pageSize int) {
	SendPageRespV2Extra(c, data, total, pageNum, pageSize, nil)
}

// SendPageRespV2Extra 分页结果返回，并附带额外字段（如列表统计概览）。
//
// list/total/pageNum/pageSize/pagesCount 结构与 SendPageRespV2 保持一致，
// extra 中的键为同级兄弟字段，不影响既有前端解析。
func SendPageRespV2Extra(c *gin.Context, data interface{}, total int64, pageNum, pageSize int, extra map[string]interface{}) {
	pagesCount := 0
	if pageSize > 0 && total > 0 {
		pagesCount = int((total + int64(pageSize) - 1) / int64(pageSize))
	}
	payload := map[string]interface{}{
		"list":       data,
		"total":      total,
		"pageNum":    pageNum,
		"pageSize":   pageSize,
		"pagesCount": pagesCount,
	}
	for key, value := range extra {
		if key == "" {
			continue
		}
		payload[key] = value
	}
	SendNormalResp(c, entity.ErrCodeOK, "success", payload)
}

// SendOKResp 正常结果返回
func SendOKResp(c *gin.Context, data interface{}) {
	SendNormalResp(c, entity.ErrCodeOK, "success", data)
}

// SendInternalResp 返回500状态码结果
func SendInternalResp(c *gin.Context) {
	c.JSON(http.StatusOK, &entity.Response{
		Code:      entity.ErrCodeInternal,
		Message:   entity.ErrMsgInternal,
		RequestID: requestid.Get(c),
	})
}

// SendNormalResp 返回200状态码结果
func SendNormalResp(c *gin.Context, code entity.ErrCode, msg string, data interface{}) {
	c.JSON(http.StatusOK, &entity.Response{
		Code:      code,
		Message:   msg,
		RequestID: requestid.Get(c),
		Data:      data,
	})
}

// SendForbiddenResp 返回403状态码结果
func SendForbiddenResp(c *gin.Context) {
	c.JSON(http.StatusForbidden, &entity.Response{
		Code:    entity.ErrCodeForbidden,
		Message: "403 Forbidden",
	})
}

func SendNoAuthResp(c *gin.Context) {
	c.JSON(http.StatusUnauthorized, &entity.Response{
		Code:    entity.ErrCodeNoAuth,
		Message: "401 Unauthorized",
	})
}

func SendLoginResp(c *gin.Context) {
	loginURL := oauth.GetInstance().GetLoginURL()
	SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", map[string]string{
		"login_url": loginURL,
	})
}

// SendSSEResp 发送SSE结果
func SendSSEResp(c *gin.Context, id, data string, resp entity.Response) {
	if c.Request.URL.Path == entity.SearchPath && id == SseIDError {
		c.Header("Skb-Code", fmt.Sprint(resp.Code))
		c.Header("Skb-Message", url.QueryEscape(resp.Message))
	}
	c.Render(-1, sse.Event{
		Id:   id,
		Data: data,
	})
	c.Writer.Flush()
}

const (
	SseIDClose = "CLOSE"
	SseIDError = "ERROR"
)

func SendSSEResponse(c *gin.Context, eventID string, data entity.Response, withClose bool) {
	data.RequestID = requestid.Get(c)
	dataStr, _ := json.Marshal(data)
	SendSSEResp(c, eventID, string(dataStr), data)
	if withClose {
		SendSSEResp(c, SseIDClose, "", data)
	}
}
