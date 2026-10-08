package storage

import (
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v4/neo4j"

	"bid-engine/lib/common/config"
	"bid-engine/lib/common/logtool"
)

// GetNeo4jDriver 获取链接
func GetNeo4jDriver(dbName string) (neo4j.Driver, error) {
	// 读取配置
	var (
		neo4jConfig config.Neo4JCfg
		logger      = logtool.GetLogger()
	)
	for _, cfg := range config.GetConfig().Neo4J {
		if cfg.Name == dbName {
			neo4jConfig = cfg
			break
		}
	}
	if neo4jConfig.Host == "" || neo4jConfig.User == "" || neo4jConfig.Password == "" {
		logger.Sugar().Errorw("neo4jConfig error", "serviceName", dbName)
		return nil, fmt.Errorf("")
	}
	driver, err := neo4j.NewDriver(neo4jConfig.Host, neo4j.BasicAuth(neo4jConfig.User, neo4jConfig.Password, ""))
	if err != nil {
		logtool.MustGetLogger().Sugar().Warn("连接neo4j数据库失败", "dbName", dbName, "err", err)
	}
	return driver, err
}
