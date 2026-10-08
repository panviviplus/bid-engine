package logtool

import (
	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc/metadata"

	"bid-engine/lib/common/config"
)

const (
	// UserIDKey ctx里面用户ID的key
	UserIDKey = "userID"
)

var logger *zap.Logger

// MustInitLogger 初始化日志实例
func MustInitLogger() {
	systemConf := config.GetConfig()
	var logLevel zapcore.Level
	switch systemConf.Log.LogLevel {
	case "debug":
		logLevel = zap.DebugLevel
	case "info":
		logLevel = zap.InfoLevel
	case "error":
		logLevel = zap.ErrorLevel
	default:
		logLevel = zap.InfoLevel
	}
	logger = NewLogger(
		SetAppName(systemConf.Server.APPName),
		SetDevelopment(true),
		SetLevel(logLevel),
		SetLogFileDir(systemConf.Log.LogDir),
		SetMaxSize(systemConf.Log.MaxSize),
		SetMaxBackups(systemConf.Log.MaxBackups),
		SetMaxAge(systemConf.Log.MaxAge),
	)
}

// GetLogger 获取日志实例
func GetLogger() *zap.Logger {
	if logger == nil {
		panic("logger没有初始化")
	}
	return logger
}

// MustGetLogger 获取日志实例
func MustGetLogger() *zap.Logger {
	if logger == nil {
		logger = zap.NewExample()
	}
	return logger
}

// GetRequestIDFromContext 通过context获取reqeustID
func GetRequestIDFromContext(ctx context.Context) string {
	srcCtx, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		srcCtx, ok = metadata.FromOutgoingContext(ctx)
		if !ok {
			return ""
		}
	}
	requestIDs := srcCtx.Get("x-request-id")
	if len(requestIDs) > 0 {
		return requestIDs[0]
	}
	return ""
}

// WithCtx 日志中添加ctx信息
// 添加traceID
func WithCtx(ctx context.Context) zap.Field {
	return zap.String("traceId", GetRequestIDFromContext(ctx))
}

// SetUserID2Ctx 设置用户UserID到ctx
func SetUserID2Ctx(ctx context.Context, userID int64) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}

// Ctx 日志中添加ctx信息
// 添加traceID和用户uid
func Ctx(ctx context.Context) []interface{} {
	userID, _ := ctx.Value(UserIDKey).(int64)
	// 从ctx中获取trace和useID
	return []interface{}{
		zap.String("traceId", GetRequestIDFromContext(ctx)),
		zap.Int64("_userID", userID),
	}
}
