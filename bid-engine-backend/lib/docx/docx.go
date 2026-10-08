package docx

import (
	"encoding/json"
	"regexp"

	"github.com/carmel/gooxml/document"
	"go.uber.org/zap"

	"bid-engine/pkg/entity"
)

var (
	headingRegex, _ = regexp.Compile(`[Hh]eading\s?(\d+)`)
)

type parser struct {
	filename        string
	logger          *zap.SugaredLogger
	styleID2heading map[string]string
	doc             *document.Document
}

func NewParser(filename string, logger *zap.SugaredLogger) *parser {
	return &parser{
		filename:        filename,
		logger:          logger,
		styleID2heading: make(map[string]string),
	}
}

func (p *parser) ToTemplate() ([]entity.Paragraph, error) {
	p.logger.Infow("Parsing docx", "filename", p.filename)
	// 打开文件
	doc, err := document.Open(p.filename)
	if err != nil {
		p.logger.Warnw("文档打开错误", "filename", p.filename)
		return nil, err
	}
	p.doc = doc
	// 新建 styleID 到 名称 的映射
	p.genStyle2heading()
	p.logger.Infow("Style2heading解析结果", "styleID2heading", p.styleID2heading)
	//return nil, nil
	lastHeading := 0
	root := &HeadingNode{Level: 0, Title: "", Children: []*HeadingNode{}}
	// 遍历段落
	for _, para := range doc.Paragraphs() {
		headingLevel := p.checkHeading(para)
		if headingLevel == 0 {
			continue
		}
		text := ""
		runs := para.Runs()
		for _, run := range runs {
			text = text + run.Text()
		}
		p.logger.Infow("heading数据", "level", headingLevel, "text", text)
		// 相当于是 H1 下面是 H3, 直接忽略
		if headingLevel-lastHeading >= 2 {
			continue
		}
		lastHeading = headingLevel
		insertHeading(root, &HeadingNode{
			Level: headingLevel,
			Title: text,
		})
	}
	treeJSON, _ := json.Marshal(root)
	p.logger.Infow("heading tree结构", "tree", string(treeJSON))
	// 转换成模板
	template := root.toTemplate()
	templateJSON, _ := json.Marshal(template)
	p.logger.Infow("template数据", "template", string(templateJSON))
	return template, nil
}

func (p *parser) genStyle2heading() {
	for _, s := range p.doc.Styles.Styles() {
		// 正则匹配一下
		matches := headingRegex.FindStringSubmatch(s.Name())
		if len(matches) == 2 {
			p.styleID2heading[s.StyleID()] = "Heading" + matches[1]
		}
	}
}

func (p *parser) checkHeading(par document.Paragraph) int {
	level := p.checkHeadingByStyle(par.Style())
	if level > 0 {
		return level
	}
	return p.checkHeadingByStyleID(par)
}

func (p *parser) checkHeadingByStyle(style string) int {
	switch style {
	case "Heading1":
		return 1
	case "Heading2":
		return 2
	case "Heading3":
		return 3
	case "Heading4":
		return 4
	case "Heading5":
		return 5
	case "Heading6":
		return 6
	case "Heading7":
		return 7
	default:
		return 0
	}
}

func (p *parser) checkHeadingByStyleID(par document.Paragraph) int {
	styleID := par.Properties().Style()
	style, ok := p.styleID2heading[styleID]
	if !ok {
		return 0
	}
	return p.checkHeadingByStyle(style)
}
