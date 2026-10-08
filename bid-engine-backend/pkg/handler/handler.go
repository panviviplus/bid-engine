package handler

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	DefaultDownloadRangeSize = 1024 * 1024 * 8
)

// GetContentRange 从context获取renge
func GetContentRange(c *gin.Context, size int64) string {
	var contentRange string
	originContentRange := c.Request.Header.Get("range")
	if originContentRange != "" {
		contentRange = strings.Trim(originContentRange, "bytes=")
		strArr := strings.Split(contentRange, "-")
		start, _ := strconv.ParseInt(strArr[0], 10, 64)
		end, _ := strconv.ParseInt(strArr[1], 10, 64)
		if start != 0 && end == 0 {
			if start+DefaultDownloadRangeSize > size {
				end = size - 1
			} else {
				end = start + DefaultDownloadRangeSize
			}
			contentRange = fmt.Sprintf("bytes=%d-%d", start, end)
		} else {
			contentRange = originContentRange
		}
	}
	return contentRange
}
