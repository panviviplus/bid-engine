package util

import (
	"context"

	"google.golang.org/grpc/metadata"
)

// GetValueFromContext 通过context获取内容
// 获取顺序 当前 ctx.value -> metadata
func GetValueFromContext(ctx context.Context, key string) string {
	ctxValue := ctx.Value(key)
	if ctxValue != nil {
		strVal, _ := ctxValue.(string)
		return strVal
	}
	srcCtx, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		srcCtx, ok = metadata.FromOutgoingContext(ctx)
		if !ok {
			return ""
		}
	}
	metaValues := srcCtx.Get(key)
	if len(metaValues) > 0 {
		return metaValues[0]
	}
	return ""
}
