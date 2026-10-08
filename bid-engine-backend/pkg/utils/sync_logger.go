package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// SyncLogger 同步任务专用日志记录器
type SyncLogger struct {
	logger   *zap.SugaredLogger
	taskType string
}

// NewSyncLogger 创建同步任务日志记录器
// taskType: "company" 或 "user"
func NewSyncLogger(taskType string) (*SyncLogger, error) {
	// 确保logs目录存在
	logsDir := "logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %w", err)
	}

	// 生成日志文件名：<taskType>_YYYYMMDDHHMMSS.log（按运行时刻粒度）
	ts := time.Now().Format("20060102150405")
	logFileName := fmt.Sprintf("%s_%s.log", taskType, ts)
	logFilePath := filepath.Join(logsDir, logFileName)

	// 配置日志轮转
	lumberjackLogger := &lumberjack.Logger{
		Filename:   logFilePath,
		MaxSize:    100, // MB
		MaxBackups: 30,  // 保留30个备份文件
		MaxAge:     30,  // 保留30天
		Compress:   true,
	}

	// 创建编码器配置
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// 创建核心
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(lumberjackLogger),
		zapcore.InfoLevel,
	)

	// 创建logger
	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	return &SyncLogger{
		logger:   logger.Sugar(),
		taskType: taskType,
	}, nil
}

// LogAPIRequest 记录API请求信息
func (sl *SyncLogger) LogAPIRequest(url, method string, requestBody interface{}) {
	sl.logger.Infow("API请求",
		"任务类型", sl.taskType,
		"请求地址", url,
		"请求方法", method,
		"请求体", requestBody,
	)
}

// LogAPIResponse 记录API响应信息
func (sl *SyncLogger) LogAPIResponse(url string, statusCode int, responseBody string, duration time.Duration) {
	sl.logger.Infow("API响应",
		"任务类型", sl.taskType,
		"请求地址", url,
		"状态码", statusCode,
		"响应体", responseBody,
		"耗时毫秒", duration.Milliseconds(),
	)
}

// LogAPIError 记录API调用错误
func (sl *SyncLogger) LogAPIError(url string, err error, context string) {
	sl.logger.Errorw("API错误",
		"任务类型", sl.taskType,
		"请求地址", url,
		"错误信息", err.Error(),
		"上下文", context,
	)
}

// LogParseError 记录响应解析错误
func (sl *SyncLogger) LogParseError(responseBody string, err error, context string) {
	sl.logger.Errorw("响应解析错误",
		"任务类型", sl.taskType,
		"响应体", responseBody,
		"错误信息", err.Error(),
		"上下文", context,
	)
}

// LogDBOperation 记录数据库操作
func (sl *SyncLogger) LogDBOperation(operation string, count int, err error) {
	if err != nil {
		sl.logger.Errorw("数据库操作失败",
			"任务类型", sl.taskType,
			"操作类型", operation,
			"影响行数", count,
			"错误信息", err.Error(),
		)
	} else {
		sl.logger.Infow("数据库操作成功",
			"任务类型", sl.taskType,
			"操作类型", operation,
			"影响行数", count,
		)
	}
}

// LogTaskStart 记录任务开始
func (sl *SyncLogger) LogTaskStart() {
	sl.logger.Infow("同步任务开始",
		"任务类型", sl.taskType,
		"开始时间", time.Now().Format(time.RFC3339),
	)
}

// LogTaskComplete 记录任务完成
func (sl *SyncLogger) LogTaskComplete(totalInserted, totalUpdated int, duration time.Duration) {
	sl.logger.Infow("同步任务完成",
		"任务类型", sl.taskType,
		"新增记录", totalInserted,
		"更新记录", totalUpdated,
		"耗时秒数", duration.Seconds(),
		"结束时间", time.Now().Format(time.RFC3339),
	)
}

// LogTaskError 记录任务错误
func (sl *SyncLogger) LogTaskError(err error, context string) {
	sl.logger.Errorw("同步任务错误",
		"任务类型", sl.taskType,
		"错误信息", err.Error(),
		"上下文", context,
		"错误时间", time.Now().Format(time.RFC3339),
	)
}

// Info 记录信息日志
func (sl *SyncLogger) Info(msg string, keysAndValues ...interface{}) {
	args := append([]interface{}{"task_type", sl.taskType}, keysAndValues...)
	sl.logger.Infow(msg, args...)
}

// Error 记录错误日志
func (sl *SyncLogger) Error(msg string, keysAndValues ...interface{}) {
	args := append([]interface{}{"task_type", sl.taskType}, keysAndValues...)
	sl.logger.Errorw(msg, args...)
}

// Warn 记录警告日志
func (sl *SyncLogger) Warn(msg string, keysAndValues ...interface{}) {
	args := append([]interface{}{"task_type", sl.taskType}, keysAndValues...)
	sl.logger.Warnw(msg, args...)
}

// Close 关闭日志记录器
func (sl *SyncLogger) Close() error {
	return sl.logger.Sync()
}

// LogJobSummary 记录一次任务的审计汇总
// 参数含义：
// - totalFetched: 本趟从外部接口获取到的总记录数（跨页累计）
// - totalFilteredToInsert: 本趟过滤后待插表的记录数（跨页累计，指本地准备插入的候选条数）
// - totalInserted: 本趟实际插表成功的记录数
// - totalUpdated: 本趟实际更新成功的记录数
// - totalInsertFailed: 本趟插表失败的记录数（估算：若批量插入失败，则计该批大小）
// - totalUpdateFailed: 本趟更新失败的记录数
func (sl *SyncLogger) LogJobSummary(totalFetched, totalFilteredToInsert, totalInserted, totalUpdated, totalInsertFailed, totalUpdateFailed int) {
	sl.logger.Infow("同步任务审计汇总",
		"任务类型", sl.taskType,
		"本趟外部总获取", totalFetched,
		"本趟过滤后待插表", totalFilteredToInsert,
		"本趟插表成功", totalInserted,
		"本趟更新成功", totalUpdated,
		"本趟插表失败", totalInsertFailed,
		"本趟更新失败", totalUpdateFailed,
		"本趟总失败", totalInsertFailed+totalUpdateFailed,
	)
}
