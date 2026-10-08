package middleware

import (
	"bytes"
	"encoding/json"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"bid-engine/lib/common/logtool"
	"bid-engine/pkg/entity"
)

type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w bodyLogWriter) Write(b []byte) (int, error) {
	if !strings.Contains(w.Header().Get("Content-Type"), "text/event-stream") {
		w.body.Write(b)
	}
	return w.ResponseWriter.Write(b)
}
func (w bodyLogWriter) WriteString(s string) (int, error) {
	if !strings.Contains(w.Header().Get("Content-Type"), "text/event-stream") {
		w.body.WriteString(s)
	}
	return w.ResponseWriter.WriteString(s)
}

// LogReqAndResp 记录请求和返回数据
func LogReqAndResp() gin.HandlerFunc {
	return func(c *gin.Context) {
		blWriter := &bodyLogWriter{
			body:           bytes.NewBufferString(""),
			ResponseWriter: c.Writer,
		}

		c.Writer = blWriter
		//开始时间
		startTime := time.Now()
		//处理请求
		c.Next()
		responseBody := blWriter.body.String()
		var (
			responseCode int32
			responseMsg  string
			responseData interface{}
			logData      []interface{}
		)
		if responseBody != "" {
			response := entity.Response{}
			err := json.Unmarshal([]byte(responseBody), &response)
			if err == nil {
				responseCode = response.Code
				responseMsg = response.Message
				responseData = response.Data
			}
		}
		if c.Request.Method == "POST" {
			_ = c.Request.ParseForm()
		}
		if strings.HasPrefix(c.Request.RequestURI, "/search") {
			responseData = ""
		}
		// 日志格式
		logData = append(logData, "req_time", startTime)
		logData = append(logData, "req_method", c.Request.Method)
		logData = append(logData, "req_uri", c.Request.RequestURI)
		logData = append(logData, "req_ua", c.Request.UserAgent())
		logData = append(logData, "req_post_data", c.Request.PostForm.Encode())
		logData = append(logData, "client_ip", c.ClientIP())
		// resp
		logData = append(logData, "resp_code", responseCode)
		logData = append(logData, "resp_msg", responseMsg)
		logData = append(logData, "resp_data", responseData)
		logData = append(logData, "cost_time", time.Since(startTime).Milliseconds())

		logtool.GetLogger().Sugar().With(entity.Ctx(c)...).Infow("请求详情", logData...)
	}
}
