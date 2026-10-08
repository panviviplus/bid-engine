// Package logtool 日志工具包，封装了zap日志和lumberjack日志切分能力
package logtool

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Options 日志相关的配置
type Options struct {
	LogFileDir  string        // 文件保存地方
	AppName     string        // 日志文件前缀
	Level       zapcore.Level // 日志等级
	MaxSize     int           // 日志文件小大（M）
	MaxBackups  int           // 最多存在多少个切片文件
	MaxAge      int           // 保存的最大天数
	Development bool          // 是否是开发模式
	zap.Config
}

// ModOptions 可选项
type ModOptions func(options *Options)

var (
	l              *Logger
	sp             = string(filepath.Separator)
	fileWS         zapcore.WriteSyncer       // 日志文件
	debugConsoleWS = zapcore.Lock(os.Stdout) // 控制台标准输出
)

// Logger 封装的日志结构
type Logger struct {
	*zap.Logger
	sync.RWMutex
	Opts      *Options `json:"opts"`
	zapConfig zap.Config
	inited    bool
}

// NewLogger 实例化新的zap日志ß
func NewLogger(mod ...ModOptions) *zap.Logger {
	l = &Logger{}
	l.Lock()
	defer l.Unlock()
	if l.inited {
		l.Info("[NewLogger] logger Inited")
		return nil
	}
	l.Opts = &Options{
		LogFileDir: "",
		AppName:    "app_log",
		Level:      zapcore.DebugLevel,
		MaxSize:    100,
		MaxBackups: 60,
		MaxAge:     30,
	}
	if l.Opts.LogFileDir == "" {
		l.Opts.LogFileDir, _ = filepath.Abs(filepath.Dir(filepath.Join(".")))
		l.Opts.LogFileDir += sp + "logs" + sp
	}
	if l.Opts.Development {
		l.zapConfig = zap.NewDevelopmentConfig()
	} else {
		l.zapConfig = zap.NewProductionConfig()
	}
	l.zapConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	if l.Opts.OutputPaths == nil || len(l.Opts.OutputPaths) == 0 {
		l.zapConfig.OutputPaths = []string{"stdout"}
	}
	if l.Opts.ErrorOutputPaths == nil || len(l.Opts.ErrorOutputPaths) == 0 {
		l.zapConfig.OutputPaths = []string{"stderr"}
	}
	for _, fn := range mod {
		fn(l.Opts)
	}
	l.zapConfig.Level.SetLevel(l.Opts.Level)
	l.init()
	l.inited = true
	l.Info("[NewLogger] success")
	return l.Logger
}

func (l *Logger) init() {
	l.setSyncers()
	var err error
	l.Logger, err = l.zapConfig.Build(l.cores())
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = l.Logger.Sync()
	}()
}

func (l *Logger) setSyncers() {
	fileWS = zapcore.AddSync(&lumberjack.Logger{
		Filename:   l.Opts.LogFileDir + sp + l.Opts.AppName + ".log",
		MaxSize:    l.Opts.MaxSize,
		MaxBackups: l.Opts.MaxBackups,
		MaxAge:     l.Opts.MaxAge,
		Compress:   true,
		LocalTime:  true,
	})
	return
}

// SetMaxSize 设置日志大小，单位：MB
func SetMaxSize(MaxSize int) ModOptions {
	return func(option *Options) {
		option.MaxSize = MaxSize
	}
}

// SetMaxBackups 设置日志最大备份数量
func SetMaxBackups(MaxBackups int) ModOptions {
	return func(option *Options) {
		option.MaxBackups = MaxBackups
	}
}

// SetMaxAge 设置日志最长保留时间,单位：天
func SetMaxAge(MaxAge int) ModOptions {
	return func(option *Options) {
		option.MaxAge = MaxAge
	}
}

// SetLogFileDir 设置日志保存目录
func SetLogFileDir(LogFileDir string) ModOptions {
	return func(option *Options) {
		option.LogFileDir = LogFileDir
	}
}

// SetAppName 设置日志应用名，最终日志文件名：appName.log
func SetAppName(AppName string) ModOptions {
	return func(option *Options) {
		option.AppName = AppName
	}
}

// SetLevel 设置日志等级，默认是Debug
func SetLevel(Level zapcore.Level) ModOptions {
	return func(option *Options) {
		option.Level = Level
	}
}

// SetDevelopment 设置是否是开发环境，开发环境会将日志同步打印到console
func SetDevelopment(Development bool) ModOptions {
	return func(option *Options) {
		option.Development = Development
	}
}

// cores 对zap日志核心组件进行封装
func (l *Logger) cores() zap.Option {
	fileEncoder := zapcore.NewJSONEncoder(l.zapConfig.EncoderConfig)
	encoderConfig := zap.NewDevelopmentEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)

	cores := []zapcore.Core{
		zapcore.NewCore(fileEncoder, fileWS, l.Opts.Level),
	}
	if l.Opts.Development {
		cores = append(cores, []zapcore.Core{
			zapcore.NewCore(consoleEncoder, debugConsoleWS, l.Opts.Level),
		}...)
	}
	return zap.WrapCore(func(c zapcore.Core) zapcore.Core {
		return zapcore.NewTee(cores...)
	})
}
