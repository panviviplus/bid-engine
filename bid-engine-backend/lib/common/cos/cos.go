package cos

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/tencentyun/cos-go-sdk-v5"

	"bid-engine/lib/common/config"
	"bid-engine/lib/common/logtool"
)

func GetCosConfig(name string) (config.CosCfg, error) {
	// 读取配置
	var (
		cosCfg config.CosCfg
	)
	for _, cfg := range config.GetConfig().Cos {
		if cfg.Name == name {
			cosCfg = cfg
			break
		}
	}
	if cosCfg.Name == "" {
		logtool.MustGetLogger().Sugar().Errorw("kafkaCfg error", "name", name)
		return cosCfg, fmt.Errorf("未找到相应配置")
	}
	return cosCfg, nil
}

// NewCosClient 创建cos 客户端
func NewCosClient(name string) (*cos.Client, error) {
	cosCfg, err := GetCosConfig(name)
	if err != nil {
		return nil, fmt.Errorf("配置文件错误")
	}

	u, _ := url.Parse(cosCfg.RawUrl)
	cu, _ := url.Parse(cosCfg.CiUrl)
	b := &cos.BaseURL{BucketURL: u, CIURL: cu}
	// 密钥
	client := cos.NewClient(b, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID:  cosCfg.SecretID,
			SecretKey: cosCfg.SecretKey,
		},
	})
	return client, nil
}
