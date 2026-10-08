package entity

import "github.com/google/uuid"

// Template 模板结构
type Template struct {
	ID           int32       `json:"id"`
	Title        string      `json:"title"`
	ArticleTitle string      `json:"article_title"`
	Template     []Paragraph `json:"template"`
}

// Paragraph 段落
type Paragraph struct {
	Title    Content     `json:"title"`
	Desc     Content     `json:"desc"`
	Children []Paragraph `json:"children,omitempty"`
}

// Content 一个段落
type Content struct {
	ID      string `json:"id"`
	Content string `json:"content"`
}

var allTemplates = []Template{
	{
		ID:           1,
		Title:        "行业关注点",
		ArticleTitle: "数据安全产业关注点",
		Template: []Paragraph{
			{
				Title: Content{ID: "first_title", Content: "数据安全产业产业介绍"},
				Desc:  Content{ID: "first_desc", Content: ""},
				Children: []Paragraph{
					{
						Title: Content{ID: uuid.NewString(), Content: "数据安全产业发展情况"},
						Desc:  Content{ID: uuid.NewString(), Content: "总结数据安全产业国家政策、市场需求、市场规模、增速、分类、行业巨头以及新兴企业等。"},
					},
					{
						Title: Content{ID: uuid.NewString(), Content: "数据安全产业特点"},
						Desc:  Content{ID: uuid.NewString(), Content: "概述行业核心技术、热点、创新性特征、业务模式、毛利率。"},
					},
				},
			},
			{
				Title: Content{ID: uuid.NewString(), Content: "行业企业分析"},
				Desc:  Content{ID: uuid.NewString(), Content: "总结行业巨头以及新兴企业、技术和产品、研发投入、收入情况； 公司亮点与存在的问题。"},
			},
			{
				Title: Content{ID: uuid.NewString(), Content: "新兴数据安全技术和趋势"},
				Desc:  Content{ID: uuid.NewString(), Content: "介绍数据安全行业的新兴技术，包括名称、时间及国内外最新情况。"},
			},
			{
				Title: Content{ID: uuid.NewString(), Content: "未来展望"},
				Desc:  Content{ID: uuid.NewString(), Content: "介绍数据安全行业未来发展趋势，分析数据安全行业收益主要来源以及利润潜在增长点。"},
			},
		},
	},
	{
		ID:           2,
		Title:        "工作总结",
		ArticleTitle: "数字化转型工作总结",
		Template: []Paragraph{
			{
				Title: Content{ID: "first_title", Content: "总体目标"},
				Desc:  Content{ID: "first_desc", Content: "总结数字化转型的目标和实施路径、方法、效果等。"},
			},
			{
				Title: Content{ID: uuid.NewString(), Content: "主要工作举措"},
				Desc: Content{ID: uuid.NewString(), Content: "总结主要工作举措：建立的工作方法论、机制、会议制度等，研究及产出的相关报告，" +
					"数智化投研系统进展，产业协同进展，生态圈构建，大赛大会参与情况，知识产权情况。"},
			},
			{
				Title: Content{ID: uuid.NewString(), Content: "突破点和亮点"},
				Desc: Content{ID: uuid.NewString(), Content: "使用的数字化、智能化技术，比如数据治理、大模型、向量数据库、隐私计算等；" +
					"技术应用的效果，比如产投协同效果、生态建设、内外部影响力、获奖情况等。"},
			},
			{
				Title: Content{ID: uuid.NewString(), Content: "主要问题"},
				Desc:  Content{ID: uuid.NewString(), Content: "工作中存在的问题，比如点状工作、投研作用的发挥、没有考虑到的技术点等。"},
			},
		},
	},
}

// GetAllTemplates 获取所有的模板
func GetAllTemplates() []Template {
	return allTemplates
}
