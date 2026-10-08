package schedule

import (
	"strings"
	"sync"
	"time"

	skbcfg "bid-engine/pkg/config"
	"bid-engine/lib/common/config"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"bid-engine/lib/common/logtool"
)

// Manager 定时任务管理器，支持通过 Cron 表达式自由注册任务
type Manager struct {
	cron   *cron.Cron
	logger *zap.SugaredLogger
	mu     sync.Mutex
	jobs   map[string]cron.EntryID // name -> entryID
}

var (
	manager     *Manager
	onceManager sync.Once
)

// Init 初始化调度器（如果未初始化），注册默认任务并启动
func Init() {
	onceManager.Do(func() {
		manager = &Manager{
			// 开启秒级任务
			cron:   cron.New(cron.WithSeconds(), cron.WithLocation(time.Local)),
			logger: logtool.GetLogger().Sugar(),
			jobs:   make(map[string]cron.EntryID),
		}
	})
	// 注册默认任务
	registerDefaultJobs()
	// 启动
	manager.Start()
}

// Start 启动调度器
func (m *Manager) Start() {
	m.logger.Infow("Scheduler start")
	m.cron.Start()
}

// Stop 停止调度器
func (m *Manager) Stop() {
	m.logger.Infow("Scheduler stop")
	m.cron.Stop()
}

// Add 注册任务；如果同名任务已存在则先移除再注册
func Add(spec string, name string, job func()) (cron.EntryID, error) {
	if manager == nil {
		Init()
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	// 如果同名任务存在，先移除
	if id, ok := manager.jobs[name]; ok {
		manager.cron.Remove(id)
		delete(manager.jobs, name)
		manager.logger.Infow("Scheduler remove existing job", "name", name, "id", id)
	}
	// 注册新任务
	id, err := manager.cron.AddFunc(spec, func() {
		manager.logger.Infow("Scheduler run job", "name", name)
		job()
	})
	if err != nil {
		manager.logger.Errorw("Scheduler add job failed", "name", name, "spec", spec, "err", err)
		return 0, err
	}
	manager.jobs[name] = id
	manager.logger.Infow("Scheduler add job ok", "name", name, "spec", spec, "id", id)
	return id, nil
}

// Remove 删除任务
func Remove(name string) {
	if manager == nil {
		return
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if id, ok := manager.jobs[name]; ok {
		manager.cron.Remove(id)
		delete(manager.jobs, name)
		manager.logger.Infow("Scheduler remove job", "name", name, "id", id)
	}
}

// registerDefaultJobs 在这里集中注册项目默认内建的任务
func registerDefaultJobs() {
	// 调度器探活任务(勿删)：每5秒打印一行日志，心跳监测
	_ = RegisterHealthJobEvery5Seconds()

	// 其他业务定时任务（按需在此注册）

}

func RegisterHealthJobEvery5Seconds() error {
	// 秒 分 时 日 月 周
	spec := "0 0 * * * *" // 整点（每小时）触发
	name := "定时任务调度器探活"
	_, err := Add(spec, name, PrintLog)
	return err
}

func PrintLog() {
	logtool.GetLogger().Sugar().Infow("==========  Scheduler heartbeat  ==========")
}

// isProdEnv 返回是否为生产环境（优先读 SKB_ENV，其次读配置 env）
func isProdEnv() bool {
	env := skbcfg.Get("env")
	if env == "" {
		env = config.GetConfig().GetProperty("env")
	}
	env = strings.ToLower(env)
	return env == "zwy"
}
