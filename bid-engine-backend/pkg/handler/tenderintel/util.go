package tenderintel

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ── 公告类型 ────────────────────────────────────────────────────

const (
	NoticeTypeOpenTender        = "open_tender"
	NoticeTypeInviteTender      = "invite_tender"
	NoticeTypeNegotiation       = "negotiation"
	NoticeTypeTenderNegotiation = "tender_negotiation"
	NoticeTypeInquiry           = "inquiry"
	NoticeTypePrequalification  = "prequalification"
	NoticeTypeSingleSource      = "single_source"
	NoticeTypeChange            = "change"
	NoticeTypeTerminate         = "terminate"
	NoticeTypeOther             = "other"

	NoticeStageProcurement      = "procurement"
	NoticeStagePrequalification = "prequalification"
	NoticeStageChange           = "change"
	NoticeStageOther            = "other"
)

// noticeTypeNames 公告类型中文名，与前端展示保持一致。
var noticeTypeNames = map[string]string{
	NoticeTypeOpenTender:        "公开招标",
	NoticeTypeInviteTender:      "邀请招标",
	NoticeTypeNegotiation:       "竞争性磋商",
	NoticeTypeTenderNegotiation: "竞争性谈判",
	NoticeTypeInquiry:           "询价",
	NoticeTypePrequalification:  "资格预审",
	NoticeTypeSingleSource:      "单一来源",
	NoticeTypeChange:            "变更澄清",
	NoticeTypeTerminate:         "终止废标",
	NoticeTypeOther:             "其他",
}

// NoticeTypeName 返回公告类型中文名。
func NoticeTypeName(code string) string {
	if name, ok := noticeTypeNames[code]; ok {
		return name
	}
	return noticeTypeNames[NoticeTypeOther]
}

// noticeTypeOrder 公告类型展示顺序（与筛选器、订阅表单的选项顺序一致）。
var noticeTypeOrder = []string{
	NoticeTypeOpenTender,
	NoticeTypeInviteTender,
	NoticeTypeNegotiation,
	NoticeTypeTenderNegotiation,
	NoticeTypeInquiry,
	NoticeTypePrequalification,
	NoticeTypeSingleSource,
	NoticeTypeChange,
	NoticeTypeTerminate,
	NoticeTypeOther,
}

// NoticeTypeOption 公告类型选项。
type NoticeTypeOption struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// NoticeTypeOptions 返回全部公告类型选项。
//
// 类型是固定枚举（与解析规则一致），不依赖“库里当前是否已有该类型的公告”——
// 否则新部署时筛选项会是空的，用户无法选择。
func NoticeTypeOptions() []NoticeTypeOption {
	out := make([]NoticeTypeOption, 0, len(noticeTypeOrder))
	for _, code := range noticeTypeOrder {
		out = append(out, NoticeTypeOption{Code: code, Name: NoticeTypeName(code)})
	}
	return out
}

// noticeTypeRules 公告类型判定规则，按顺序匹配，先命中者生效。
var noticeTypeRules = []struct {
	keywords []string
	code     string
	stage    string
}{
	{[]string{"终止公告", "废标", "流标", "终止招标"}, NoticeTypeTerminate, NoticeStageOther},
	{[]string{"变更公告", "澄清", "更正公告", "补充公告", "答疑"}, NoticeTypeChange, NoticeStageChange},
	{[]string{"资格预审"}, NoticeTypePrequalification, NoticeStagePrequalification},
	{[]string{"单一来源"}, NoticeTypeSingleSource, NoticeStageProcurement},
	{[]string{"竞争性磋商", "磋商公告"}, NoticeTypeNegotiation, NoticeStageProcurement},
	{[]string{"竞争性谈判", "谈判公告"}, NoticeTypeTenderNegotiation, NoticeStageProcurement},
	{[]string{"询价公告", "询价采购"}, NoticeTypeInquiry, NoticeStageProcurement},
	{[]string{"邀请招标"}, NoticeTypeInviteTender, NoticeStageProcurement},
	{[]string{"公开招标", "招标公告", "采购公告", "招标采购"}, NoticeTypeOpenTender, NoticeStageProcurement},
}

// ClassifyNoticeType 依据标题与来源站类型文本判定公告类型与阶段。
func ClassifyNoticeType(title, rawType string) (code, stage string) {
	text := title + " " + rawType
	for _, rule := range noticeTypeRules {
		for _, kw := range rule.keywords {
			if strings.Contains(text, kw) {
				return rule.code, rule.stage
			}
		}
	}
	return NoticeTypeOther, NoticeStageOther
}

// IsProcurementNotice 判断是否为“采购阶段”公告（招标情报站只收录这类公告）。
func IsProcurementNotice(code string) bool {
	switch code {
	case NoticeTypeOpenTender, NoticeTypeInviteTender, NoticeTypeNegotiation,
		NoticeTypeTenderNegotiation, NoticeTypeInquiry, NoticeTypePrequalification,
		NoticeTypeSingleSource, NoticeTypeChange, NoticeTypeTerminate:
		return true
	default:
		return false
	}
}

// NoticeStageFor 返回公告类型对应的公告阶段（手工录入时用于补全 notice_stage）。
func NoticeStageFor(code string) string {
	switch code {
	case NoticeTypePrequalification:
		return NoticeStagePrequalification
	case NoticeTypeChange:
		return NoticeStageChange
	case NoticeTypeTerminate, NoticeTypeOther:
		return NoticeStageOther
	default:
		return NoticeStageProcurement
	}
}

// ── URL 规范化与哈希 ────────────────────────────────────────────

// trackingParams 需要从规范化链接中剔除的跟踪参数。
var trackingParams = map[string]struct{}{
	"utm_source": {}, "utm_medium": {}, "utm_campaign": {}, "utm_term": {}, "utm_content": {},
	"from": {}, "spm": {}, "share_token": {}, "ref": {},
}

// CanonicalizeURL 规范化链接：统一 host 小写、去 fragment、去跟踪参数、查询参数排序。
func CanonicalizeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Fragment = ""

	q := u.Query()
	for key := range q {
		if _, ok := trackingParams[strings.ToLower(key)]; ok {
			q.Del(key)
		}
	}
	u.RawQuery = q.Encode()

	// 去掉根路径多余的尾部斜杠差异（保留非根路径的尾斜杠作为站点惯例）
	if u.Path == "/" {
		u.Path = ""
	}
	return u.String()
}

// HashURL 计算规范化链接的 SHA-256，作为公告唯一键。
func HashURL(canonical string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(canonical)))
	return hex.EncodeToString(sum[:])
}

// HashContent 计算正文内容哈希，用于判断公告内容是否更新。
func HashContent(text string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(text)))
	return hex.EncodeToString(sum[:])
}

// ── 省份识别 ────────────────────────────────────────────────────

var provinceNames = []string{
	"北京市", "天津市", "上海市", "重庆市",
	"河北省", "山西省", "辽宁省", "吉林省", "黑龙江省",
	"江苏省", "浙江省", "安徽省", "福建省", "江西省", "山东省",
	"河南省", "湖北省", "湖南省", "广东省", "海南省",
	"四川省", "贵州省", "云南省", "陕西省", "甘肃省", "青海省",
	"内蒙古自治区", "广西壮族自治区", "西藏自治区", "宁夏回族自治区", "新疆维吾尔自治区",
	"香港特别行政区", "澳门特别行政区", "台湾省",
}

var provinceShort = map[string]string{
	"北京": "北京市", "天津": "天津市", "上海": "上海市", "重庆": "重庆市",
	"河北": "河北省", "山西": "山西省", "辽宁": "辽宁省", "吉林": "吉林省", "黑龙江": "黑龙江省",
	"江苏": "江苏省", "浙江": "浙江省", "安徽": "安徽省", "福建": "福建省", "江西": "江西省", "山东": "山东省",
	"河南": "河南省", "湖北": "湖北省", "湖南": "湖南省", "广东": "广东省", "海南": "海南省",
	"四川": "四川省", "贵州": "贵州省", "云南": "云南省", "陕西": "陕西省", "甘肃": "甘肃省", "青海": "青海省",
	"内蒙古": "内蒙古自治区", "广西": "广西壮族自治区", "西藏": "西藏自治区",
	"宁夏": "宁夏回族自治区", "新疆": "新疆维吾尔自治区",
}

// ExtractProvince 从文本中识别省级地区，无法识别时返回空串。
func ExtractProvince(text string) string {
	if text == "" {
		return ""
	}
	for _, name := range provinceNames {
		if strings.Contains(text, name) {
			return name
		}
	}
	for short, full := range provinceShort {
		if strings.Contains(text, short) {
			return full
		}
	}
	return ""
}

// ProvinceList 返回省级地区枚举（筛选器与订阅表单的选项来源）。
func ProvinceList() []string {
	out := make([]string, len(provinceNames))
	copy(out, provinceNames)
	return out
}

// ── 金额与日期解析 ──────────────────────────────────────────────

var budgetPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(?:预算|最高限价|控制价|金额|限价)[^0-9]{0,12}([0-9][0-9,，]*(?:\.[0-9]+)?)\s*(亿元|万元|万|元)`),
	regexp.MustCompile(`([0-9][0-9,，]*(?:\.[0-9]+)?)\s*(亿元|万元|万|元)`),
}

// ParseBudgetAmount 从文本中解析预算金额（单位：元）。
func ParseBudgetAmount(text string) *float64 {
	if text == "" {
		return nil
	}
	for _, pattern := range budgetPatterns {
		match := pattern.FindStringSubmatch(text)
		if len(match) < 3 {
			continue
		}
		value := strings.NewReplacer(",", "", "，", "").Replace(match[1])
		amount, err := strconv.ParseFloat(value, 64)
		if err != nil {
			continue
		}
		switch match[2] {
		case "亿元":
			amount *= 100000000
		case "万元", "万":
			amount *= 10000
		}
		if amount <= 0 {
			continue
		}
		return &amount
	}
	return nil
}

var datePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(\d{4})[-/年](\d{1,2})[-/月](\d{1,2})`),
}

// ParseDate 从文本中解析日期（取首个匹配）。
func ParseDate(text string) *time.Time {
	for _, pattern := range datePatterns {
		match := pattern.FindStringSubmatch(text)
		if len(match) < 4 {
			continue
		}
		year, err1 := strconv.Atoi(match[1])
		month, err2 := strconv.Atoi(match[2])
		day, err3 := strconv.Atoi(match[3])
		if err1 != nil || err2 != nil || err3 != nil {
			continue
		}
		if month < 1 || month > 12 || day < 1 || day > 31 {
			continue
		}
		t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.Local)
		return &t
	}
	return nil
}

// ParseDateTime 从文本中解析“日期 + 时间”，失败时退化为日期。
func ParseDateTime(text string) *time.Time {
	datetimePattern := regexp.MustCompile(`(\d{4})[-/年](\d{1,2})[-/月](\d{1,2})[日]?\s*(\d{1,2})[:：](\d{1,2})`)
	if match := datetimePattern.FindStringSubmatch(text); len(match) >= 6 {
		year, err1 := strconv.Atoi(match[1])
		month, err2 := strconv.Atoi(match[2])
		day, err3 := strconv.Atoi(match[3])
		hour, err4 := strconv.Atoi(match[4])
		minute, err5 := strconv.Atoi(match[5])
		if err1 == nil && err2 == nil && err3 == nil && err4 == nil && err5 == nil {
			t := time.Date(year, time.Month(month), day, hour, minute, 0, 0, time.Local)
			return &t
		}
	}
	return ParseDate(text)
}

// ── JSON 文本字段工具（DB 中以 TEXT 存储 JSON 数组）──────────────

// DecodeStringList 解析 DB 中的 JSON 字符串数组，非法内容返回空切片。
func DecodeStringList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return []string{}
	}
	return out
}

// EncodeStringList 序列化字符串切片为 JSON，去重并保持顺序。
func EncodeStringList(items []string) string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	if len(out) == 0 {
		return "[]"
	}
	data, err := json.Marshal(out)
	if err != nil {
		return "[]"
	}
	return string(data)
}

// sortStringsDesc 稳定倒序（内部使用）。
func sortStringsDesc(items []string) []string {
	sort.Sort(sort.Reverse(sort.StringSlice(items)))
	return items
}
