//go:build ignore
// +build ignore

// test_db.go — 数据库连接与配置加载验证脚本
//
// 用法：在项目根目录运行
//   cd bid-engine-backend/
//   go run ../tests/test_db.go
//
// 或指定配置文件：
//   CONF_FILE=./conf/conf-local.yml go run ../tests/test_db.go

package main

import (
	"fmt"
	"os"
	"strings"

	commoncfg "bid-engine/lib/common/config"
	"bid-engine/lib/common/storage"
)

func main() {
	fmt.Println("=== 标擎 — 数据库连接测试 ===\n")

	// 1. 加载配置
	configFile := resolveConfigFile()
	if configFile == "" {
		fmt.Println("❌ 未找到配置文件，请设置环境变量 CONF_FILE 或将脚本放在 bid-engine-backend/ 下运行")
		os.Exit(1)
	}
	fmt.Printf("📄 配置文件: %s\n", configFile)
	commoncfg.MustLoadConfig(configFile)
	fmt.Println("✅ 配置加载成功")

	// 2. 验证 DSN
	cfg := commoncfg.GetConfig()
	dsn := cfg.Mysql.DSN
	if dsn == "" {
		fmt.Println("❌ 未找到 mysql.dsn 配置项")
		os.Exit(1)
	}
	fmt.Printf("🔗 DSN: %s\n", maskDSN(dsn))

	// 3. 连接数据库
	fmt.Println("\n⏳ 正在连接数据库...")
	storage.MustInitDB()
	db := storage.GetDB()

	// 4. 验证连接
	sqlDB, err := db.DB()
	if err != nil {
		fmt.Printf("❌ 获取底层 sql.DB 失败: %v\n", err)
		os.Exit(1)
	}
	if err := sqlDB.Ping(); err != nil {
		fmt.Printf("❌ 数据库 Ping 失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ 数据库连接成功")

	// 5. 执行简单查询验证
	var result int
	if err := db.Raw("SELECT 1").Scan(&result).Error; err != nil {
		fmt.Printf("❌ 查询验证失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ 查询验证通过 (SELECT 1 = %d)\n", result)

	// 6. 输出连接池信息
	stats := sqlDB.Stats()
	fmt.Printf("\n📊 连接池状态:\n")
	fmt.Printf("   空闲连接: %d\n", stats.Idle)
	fmt.Printf("   使用中:   %d\n", stats.InUse)
	fmt.Printf("   最大打开: %d\n", stats.MaxOpenConnections)
	fmt.Printf("   等待计数: %d\n", stats.WaitCount)

	fmt.Println("\n🎉 数据库连接测试全部通过！")
}

// resolveConfigFile 按优先级查找配置文件
func resolveConfigFile() string {
	if f := os.Getenv("CONF_FILE"); f != "" {
		return f
	}
	candidates := []string{"./conf/conf-local.yml", "../bid-engine-backend/conf/conf-local.yml", "/service/conf/conf.yml"}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// maskDSN 隐藏 DSN 中的密码信息（MySQL user:password@tcp(...)/db 格式）
func maskDSN(dsn string) string {
	atIdx := strings.LastIndex(dsn, "@")
	if atIdx < 0 {
		return dsn
	}
	colonIdx := strings.Index(dsn, ":")
	if colonIdx > 0 && colonIdx < atIdx {
		return dsn[:colonIdx+1] + "***" + dsn[atIdx:]
	}
	return dsn
}
