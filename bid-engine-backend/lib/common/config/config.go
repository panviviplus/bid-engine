package config

import (
	"io/ioutil"
	"strings"
	"sync"

	"gopkg.in/yaml.v2"

	"bid-engine/lib/common/confoverride"
)

var (
	// systemConf 系统配置
	systemConf *Config
	once       sync.Once
)

// Config 系统配置结构
type Config struct {
	Env        string            `yaml:"env"`        // 全局配置
	Server     ServerCfg         `yaml:"server"`     // 服务相关配置
	Log        LogCfg            `yaml:"log"`        // 日志相关配置
	Mysql      MysqlCfg          `yaml:"mysql"`      // Mysql相关配置
	DB         []DBCfg           `yaml:"db"`         // 数据库配置 可替代上面的Mysql
	Neo4J      []Neo4JCfg        `yaml:"neo4j"`      // neo4j图数据库配置 可替代上面的Mysql
	Redis      RedisCfg          `yaml:"redis"`      // Redis相关配置
	Kafka      []KafkaCfg        `yaml:"kafka"`      // Kafka相关配置
	ES         []ESCfg           `yaml:"es"`         // ES相关配置
	Service    []ServiceCfg      `yaml:"service"`    // 调用的第三方服务
	Cos        []CosCfg          `yaml:"cos"`        // cos相关配置
	Minio      MinioCfg          `yaml:"minio"`      // minio对象存储配置
	Meeting    *MeetingCfg       `yaml:"meeting"`    // 视频会议相关配置
	Pay        *PayCfg           `yaml:"pay"`        // 支付相关配置
	SCF        SCFCfg            `yaml:"scf"`        // 云函数配置
	OAuth      []OAuthCfg        `yaml:"oauth"`      // oauth相关配置
	Properties map[string]string `yaml:"properties"` // 自定义属性
	Apollo     *ApolloCfg        `yaml:"apollo"`     // Apollo 配置中心配置
	RawRank    *RawRankCfg       `yaml:"raw_rank"`   // 粗排配置
}

// ServerCfg 服务相关配置
type ServerCfg struct {
	APPName  string `yaml:"app_name"`  // 服务的名称
	BinPath  string `yaml:"bin_path"`  // 可执行文件目录
	GrpcPort string `yaml:"grpc_port"` // GRPC服务端口
	HTTPPort string `yaml:"http_port"` // HTTP服务端口
	BaseURL  string `yaml:"base_url"`  // HTTP服务域名
}

// LogCfg 日志相关配置
type LogCfg struct {
	LogDir     string `yaml:"log_dir"`     // 日志目录
	LogLevel   string `yaml:"log_level"`   // 日志等级
	MaxSize    int    `yaml:"max_size"`    // 日志大小（单位MB）
	MaxBackups int    `yaml:"max_backups"` // 日志保留最多备份数量
	MaxAge     int    `yaml:"max_age"`     // 日志最多保留天数
}

// MysqlCfg Mysql数据库链接配置
type MysqlCfg struct {
	DSN string `yaml:"dsn"` // 数据库链接配置
}

// DBCfg 数据库链接配置,比MysqlCfg更灵活
type DBCfg struct {
	Name string `yaml:"name"` // 服务的名称
	DSN  string `yaml:"dsn"`  // 数据库链接配置
}

// Neo4JCfg 图数据库配置
type Neo4JCfg struct {
	Name     string `yaml:"name"`     // 服务的名称
	Host     string `yaml:"host"`     // 地址
	User     string `yaml:"user"`     // 用户名
	Password string `yaml:"password"` // 密码
}

// RedisCfg Redis链接配置
type RedisCfg struct {
	Host string `yaml:"host"` // redis地址 ip:port
	Auth string `yaml:"auth"` // 密码
}

// ServiceCfg 服务配置
type ServiceCfg struct {
	Name string `yaml:"name"` // 服务的名称
	Host string `yaml:"host"` // 服务的地址
}

// MeetingCfg 视频会议配置
type MeetingCfg struct {
	AsrType       string `yaml:"asr_type"`         // asr类型
	FeiShuWebHook string `yaml:"fei_shu_web_hook"` // webhook 地址
}

// PayCfg 支付相关配置
type PayCfg struct {
	AlipayNotifyUrl string `yaml:"alipay_notify_url"` // 支付宝回调url
	AlipayReturnUrl string `yaml:"alipay_return_url"` // 支付宝返回url
	WechatNotifyUrl string `yaml:"wechat_notify_url"` // 微信支付回调url
}

// KafkaCfg kafka配置
type KafkaCfg struct {
	Name       string   `yaml:"name"`        // 服务的名称
	Hosts      []string `yaml:"hosts"`       // 服务的地址
	InstanceID string   `yaml:"instance_id"` // 实例ID
	Sasl       bool     `yaml:"sasl"`        // 密码验证
	User       string   `yaml:"user"`        // 用户名
	Password   string   `yaml:"password"`    // 密码
}

// CosCfg cos配置
type CosCfg struct {
	Name      string `yaml:"name"`       // 服务的名称
	RawUrl    string `yaml:"raw_url"`    // url
	CiUrl     string `yaml:"ci_url"`     // CI url
	SecretID  string `yaml:"secret_id"`  // id
	SecretKey string `yaml:"secret_key"` // key
}

// MinioCfg MinIO对象存储配置
type MinioCfg struct {
	Endpoint      string `yaml:"endpoint"`       // MinIO 服务端点 (例: localhost:9000)
	AccessKey     string `yaml:"access_key"`     // 访问密钥
	SecretKey     string `yaml:"secret_key"`     // 秘密密钥
	DefaultBucket string `yaml:"default_bucket"` // 默认桶名
	CdnBucket     string `yaml:"cdn_bucket"`     // CDN/公开桶名
	UseSSL        bool   `yaml:"use_ssl"`        // 是否使用 HTTPS
}

// OAuthCfg oauth登录配置
type OAuthCfg struct {
	Name         string `yaml:"name"`          // 三方名称
	ClientID     string `yaml:"client_id"`     // clientID
	ClientSecret string `yaml:"client_secret"` // client密钥
}

// ESCfg es配置
type ESCfg struct {
	Name     string   `yaml:"name"`     // 服务的名称
	Hosts    []string `yaml:"hosts"`    // 服务的地址
	User     string   `yaml:"user"`     // 用户名
	Password string   `yaml:"password"` // 密码
	Insecure bool     `yaml:"insecure"` // https校验
}

// SCFCfg 云函数配置
type SCFCfg struct {
	BaseURL   string `yaml:"base_url"`   // 根域名
	AppKey    string `yaml:"app_key"`    // 应用key
	AppSecret string `yaml:"app_secret"` // 应用密钥
}

// ApolloCfg 配置中心相关配置
type ApolloCfg struct {
	AppID             string `yaml:"app_id"`
	Cluster           string `yaml:"cluster"`
	NamespaceName     string `yaml:"namespace_name"`
	IP                string `yaml:"ip"`
	IsBackupConfig    bool   `default:"true" yaml:"is_backup_config"`
	BackupConfigPath  string `yaml:"backup_config_path"`
	Secret            string `yaml:"secret"`
	Label             string `yaml:"label"`
	SyncServerTimeout int    `yaml:"sync_server_timeout"`
	MustStart         bool   `default:"false" yaml:"must_start"` // MustStart 可用于控制第一次同步必须成功
}

type RawRankCfg struct {
	KnnMinCount      int     `yaml:"knn_min_count" default:"25"`       // knn 保留最小数量
	KeywordMinCount  int     `yaml:"keyword_min_count" default:"7"`    // keyword 保留最小数量
	KnnMaxCount      int     `yaml:"knn_max_count" default:"1000"`     // knn 保留最大数量
	KeywordMaxCount  int     `yaml:"keyword_max_count" default:"1000"` // keyword 保留最大数量
	KnnThreshold     float64 `yaml:"knn_threshold" default:"0.6"`      // knn 阈值
	KeywordThreshold float64 `yaml:"keyword_threshold" default:"18.0"` // keyword 阈值
}

// MustLoadConfig 载入配置文件
func MustLoadConfig(configFile string) {
	once.Do(func() {
		systemConf = &Config{}
		// 读取 yaml 配置文件
		c, err := ioutil.ReadFile(configFile)
		if err != nil {
			panic(err.Error())
		}
		// 合并本地覆盖配置（conf-*.override.yml，不纳入版本管理），用于内网地址等环境相关配置
		merged := confoverride.MergeBytes(configFile, c)
		err = yaml.Unmarshal(merged, systemConf)
		if err != nil {
			// 解析失败，读取默认配置
			panic(err.Error())
		}
	})
}

// GetConfig 获取配置信息
func GetConfig() *Config {
	if systemConf == nil {
		panic("config没有初始化")
	}
	return systemConf
}

// SetConfig 设置配置信息-单元测试使用
func SetConfig(conf Config) {
	systemConf = &conf
}

// GetProperty 获取 properties 中 key 的 value
func (c Config) GetProperty(key string) string {
	if c.Properties == nil {
		return ""
	}
	val, ok := c.Properties[key]
	if !ok {
		return ""
	}
	return strings.Trim(val, "\t\n ")
}

// GetOAuthConfig 获取oauth配置
func (c Config) GetOAuthConfig(name string) OAuthCfg {
	for _, cfg := range c.OAuth {
		if cfg.Name == name {
			return cfg
		}
	}
	return OAuthCfg{}
}

// IsEmpty 配置是否为空
func (o OAuthCfg) IsEmpty() bool {
	return o.ClientID == ""
}
