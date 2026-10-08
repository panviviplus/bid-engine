package utils

import (
	"embed"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"bid-engine/pkg/entity"

	"github.com/disintegration/imaging"
	"go.uber.org/zap"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

var fontFS embed.FS

// Operation 通用操作结构体，包含所有可能的字段
type Operation struct {
	ID   string
	Type string // Text, Rect, Ellipse, Pen, Line, Arrow

	// 通用属性
	X               float64
	Y               float64
	Width           float64
	Height          float64
	Stroke          string
	StrokeWidth     float64
	StrokeWidthOrig float64
	StrokeWidthX    float64
	StrokeWidthY    float64
	Fill            string
	Opacity         float64

	// 文本属性
	Text       string
	FontFamily string
	FontSize   float64
	Align      string
	FontStyle  string

	// 矩形属性
	CornerRadius float64

	// 椭圆属性
	RadiusX float64
	RadiusY float64

	// 缩放属性
	ScaleX      float64
	ScaleY      float64
	GlobalScale float64

	// 线条/笔迹/箭头属性
	Points  []float64
	LineCap string
}

var (
	// 缓存解析后的字体对象，key为字体路径或标识符
	parsedFontCache = make(map[string]*opentype.Font)
	fontMutex       sync.RWMutex
)

// getParsedFontFromData 从数据解析字体（带缓存）
func getParsedFontFromData(id string, data []byte, fontStyle string) (*opentype.Font, error) {
	cacheKey := fmt.Sprintf("%s_%s", id, fontStyle)

	fontMutex.RLock()
	if f, ok := parsedFontCache[cacheKey]; ok {
		fontMutex.RUnlock()
		return f, nil
	}
	fontMutex.RUnlock()

	var tt *opentype.Font
	var err error

	// 尝试作为集合解析
	ttc, err := opentype.ParseCollection(data)
	if err == nil {
		fontIndex := 0
		if strings.Contains(strings.ToLower(fontStyle), "bold") || strings.Contains(strings.ToLower(fontStyle), "粗") {
			if ttc.NumFonts() > 1 {
				fontIndex = 1
			}
		}
		if fontIndex < ttc.NumFonts() {
			tt, err = ttc.Font(fontIndex)
		}
	}

	// 尝试作为单个字体解析
	if tt == nil {
		tt, err = opentype.Parse(data)
	}

	if err != nil {
		return nil, err
	}

	fontMutex.Lock()
	parsedFontCache[cacheKey] = tt
	fontMutex.Unlock()

	return tt, nil
}

// loadFont 加载字体 face
func loadFont(size float64, fontStyle string) (font.Face, error) {
	// 1. 优先尝试加载内置字体 (embed)
	entries, err := fontFS.ReadDir("fonts")
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() || entry.Name() == "README.md" {
				continue
			}
			path := "fonts/" + entry.Name()
			data, err := fontFS.ReadFile(path)
			if err != nil {
				continue
			}

			tt, err := getParsedFontFromData("embed:"+path, data, fontStyle)
			if err != nil {
				continue
			}

			face, err := opentype.NewFace(tt, &opentype.FaceOptions{
				Size:    size,
				DPI:     72,
				Hinting: font.HintingFull,
			})
			if err == nil {
				return face, nil
			}
		}
	}

	// 2. 尝试加载系统字体（作为 fallback）
	fontPaths := []string{
		"/System/Library/Fonts/PingFang.ttc",
		"/System/Library/Fonts/STHeiti Light.ttc",
		"/System/Library/Fonts/STHeiti Medium.ttc",
		"/System/Library/Fonts/Supplemental/Songti.ttc",
		"/System/Library/Fonts/Supplemental/Kaiti.ttc",
		"/usr/share/fonts/truetype/droid/DroidSansFallbackFull.ttf",
		"/usr/share/fonts/truetype/wqy/wqy-microhei.ttc",
		"/usr/share/fonts/truetype/wqy/wqy-zenhei.ttc",
		"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/noto-cjk/NotoSansCJK-Regular.ttc",
		"./assets/fonts/PingFang.ttc",
		"./fonts/PingFang.ttc",
	}

	for _, path := range fontPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		tt, err := getParsedFontFromData("file:"+path, data, fontStyle)
		if err != nil {
			continue
		}

		face, err := opentype.NewFace(tt, &opentype.FaceOptions{
			Size:    size,
			DPI:     72,
			Hinting: font.HintingFull,
		})
		if err == nil {
			return face, nil
		}
	}
	return nil, fmt.Errorf("no font found")
}

// 辅助函数
func parseColor(hex string) (color.RGBA, error) {
	hex = strings.TrimPrefix(hex, "#")
	if hex == "transparent" {
		return color.RGBA{0, 0, 0, 0}, nil
	}
	if len(hex) == 6 {
		r, _ := strconv.ParseUint(hex[0:2], 16, 8)
		g, _ := strconv.ParseUint(hex[2:4], 16, 8)
		b, _ := strconv.ParseUint(hex[4:6], 16, 8)
		return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}, nil
	}
	return color.RGBA{}, fmt.Errorf("invalid color: %s", hex)
}

func applyOpacity(c color.RGBA, opacity float64) color.RGBA {
	if opacity >= 1.0 {
		return c
	}
	return color.RGBA{
		R: c.R,
		G: c.G,
		B: c.B,
		A: uint8(float64(c.A) * opacity),
	}
}

func round(v float64) int {
	return int(math.Round(v))
}

// 绘图函数
func drawText(img draw.Image, op Operation) {
	if op.Text == "" {
		return
	}

	fontSize := op.FontSize
	if fontSize <= 0 {
		fontSize = 14
	}

	face, err := loadFont(fontSize, op.FontStyle)
	if err != nil {
		face = basicfont.Face7x13
	}

	// 颜色处理
	fillCol, _ := parseColor(op.Fill)
	fillCol = applyOpacity(fillCol, op.Opacity)

	strokeCol, _ := parseColor(op.Stroke)
	strokeCol = applyOpacity(strokeCol, op.Opacity)

	// 计算基线
	metrics := face.Metrics()
	ascent := metrics.Ascent.Ceil()
	lineHeight := metrics.Height.Ceil() // 使用字体原本的行高
	if lineHeight == 0 {
		lineHeight = int(fontSize * 1.2)
	}

	// 处理多行文本
	lines := strings.Split(op.Text, "\n")
	baseX := round(op.X)
	baseY := round(op.Y) + ascent

	d := &font.Drawer{Face: face}

	for i, line := range lines {
		if line == "" {
			continue
		}

		y := baseY + i*lineHeight
		x := baseX

		textWidth := d.MeasureString(line).Ceil()

		switch strings.ToLower(op.Align) {
		case "center":
			x -= textWidth / 2
		case "right":
			x -= textWidth
		}

		// 描边 (优化算法：8方向偏移)
		if op.StrokeWidth > 0 && op.Stroke != "transparent" {
			strokeWidth := int(op.StrokeWidth + 0.5)
			if strokeWidth > 0 {
				dirs := []struct{ dx, dy int }{
					{-1, 0}, {1, 0}, {0, -1}, {0, 1},
					{-1, -1}, {1, -1}, {-1, 1}, {1, 1},
				}
				step := 1
				if strokeWidth > 5 {
					step = 2
				}
				for r := 1; r <= strokeWidth; r += step {
					for _, dir := range dirs {
						d.Dst = img
						d.Src = image.NewUniform(strokeCol)
						d.Dot = fixed.P(x+dir.dx*r, y+dir.dy*r)
						d.DrawString(line)
					}
				}
			}
		}

		// 填充
		d.Dst = img
		d.Src = image.NewUniform(fillCol)
		d.Dot = fixed.P(x, y)
		d.DrawString(line)
	}
}

func drawRect(img draw.Image, op Operation) {
	x, y := round(op.X), round(op.Y)
	w, h := round(op.Width), round(op.Height)
	strokeWidth := round(op.StrokeWidth)
	radius := round(op.CornerRadius)

	// 限制圆角半径不超过宽高的一半
	if radius > w/2 {
		radius = w / 2
	}
	if radius > h/2 {
		radius = h / 2
	}

	// 颜色
	fillCol, _ := parseColor(op.Fill)
	fillCol = applyOpacity(fillCol, op.Opacity)
	strokeCol, _ := parseColor(op.Stroke)
	strokeCol = applyOpacity(strokeCol, op.Opacity)

	// 填充
	if op.Fill != "transparent" {
		if radius > 0 {
			// 绘制圆角矩形填充
			// 1. 中间十字矩形
			draw.Draw(img, image.Rect(x+radius, y, x+w-radius, y+h), image.NewUniform(fillCol), image.Point{}, draw.Over)
			draw.Draw(img, image.Rect(x, y+radius, x+w, y+h-radius), image.NewUniform(fillCol), image.Point{}, draw.Over)
			// 2. 四个圆角 (实心圆)
			drawCircle(img, x+radius, y+radius, radius, fillCol)     // 左上
			drawCircle(img, x+w-radius, y+radius, radius, fillCol)   // 右上
			drawCircle(img, x+radius, y+h-radius, radius, fillCol)   // 左下
			drawCircle(img, x+w-radius, y+h-radius, radius, fillCol) // 右下

			//fmt.Println("++++++++++++++++++")
		} else {
			draw.Draw(img, image.Rect(x, y, x+w, y+h), image.NewUniform(fillCol), image.Point{}, draw.Over)
			//fmt.Println("*****************************")
		}
	}

	//fmt.Println("=====================")
	//fmt.Println(x, y, x+w, y+h, fillCol)

	// 描边
	if strokeWidth > 0 && op.Stroke != "transparent" {
		if radius > 0 {
			// 绘制圆角矩形描边
			// 直线部分
			drawThickLine(img, x+radius, y, x+w-radius, y, strokeWidth, strokeCol)     // 上
			drawThickLine(img, x+radius, y+h, x+w-radius, y+h, strokeWidth, strokeCol) // 下
			drawThickLine(img, x, y+radius, x, y+h-radius, strokeWidth, strokeCol)     // 左
			drawThickLine(img, x+w, y+radius, x+w, y+h-radius, strokeWidth, strokeCol) // 右
			// 圆角部分 (空心圆弧) - 这里简化为画实心圆然后中间扣除？不行，会覆盖背景。
			// 使用 Bresenham 算法画圆弧
			drawArc(img, x+radius, y+radius, radius, strokeWidth, strokeCol, 2, 3)     // 左上
			drawArc(img, x+w-radius, y+radius, radius, strokeWidth, strokeCol, 3, 0)   // 右上
			drawArc(img, x+w-radius, y+h-radius, radius, strokeWidth, strokeCol, 0, 1) // 右下
			drawArc(img, x+radius, y+h-radius, radius, strokeWidth, strokeCol, 1, 2)   // 左下
		} else {
			//fmt.Println("!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!")
			// 居中描边
			//halfStroke := strokeWidth / 2

			halfStrokeX := round(op.StrokeWidthOrig*op.GlobalScale*op.ScaleX) / 2
			halfStrokeY := round(op.StrokeWidthOrig*op.GlobalScale*op.ScaleY) / 2

			//fmt.Println(x, y, w, h, halfStroke)
			//fmt.Println(x-halfStroke, y-halfStroke, x+w+halfStroke, y+halfStroke+1)
			//fmt.Println(x-halfStroke, y+h-halfStroke, x+w+halfStroke, y+h+halfStroke+1)
			//fmt.Println(x-halfStroke, y-halfStroke, x+halfStroke+1, y+h+halfStroke)
			//fmt.Println(x+w-halfStroke, y-halfStroke, x+w+halfStroke+1, y+halfStroke+1)

			draw.Draw(img, image.Rect(x-halfStrokeX, y-halfStrokeY, x+w+halfStrokeX, y+halfStrokeY+1), image.NewUniform(strokeCol), image.Point{}, draw.Over)
			draw.Draw(img, image.Rect(x-halfStrokeX, y+h-halfStrokeY, x+w+halfStrokeX, y+h+halfStrokeY+1), image.NewUniform(strokeCol), image.Point{}, draw.Over)
			draw.Draw(img, image.Rect(x-halfStrokeX, y-halfStrokeY, x+halfStrokeX+1, y+h+halfStrokeY), image.NewUniform(strokeCol), image.Point{}, draw.Over)
			draw.Draw(img, image.Rect(x+w-halfStrokeX, y-halfStrokeY, x+w+halfStrokeX+1, y+h+halfStrokeY), image.NewUniform(strokeCol), image.Point{}, draw.Over)
		}
	}
}

// 辅助绘制实心圆
func drawCircle(img draw.Image, cx, cy, r int, c color.Color) {
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			if dx*dx+dy*dy <= r*r {
				img.Set(cx+dx, cy+dy, c)
			}
		}
	}
}

// 辅助绘制圆弧 (quadrant: 0=右下, 1=左下, 2=左上, 3=右上)
// 这是一个简化的圆弧绘制，遍历角度
func drawArc(img draw.Image, cx, cy, r, width int, c color.Color, startQ, endQ int) {
	// 简单实现：遍历对应象限的角度
	startAngle := float64(startQ) * math.Pi / 2
	endAngle := float64(endQ) * math.Pi / 2
	if endQ == 0 && startQ == 3 {
		endAngle = 2 * math.Pi
	} // 处理跨越 0 度

	step := 1.0 / float64(r)
	for t := startAngle; t <= endAngle; t += step {
		x := float64(cx) + float64(r)*math.Cos(t)
		y := float64(cy) + float64(r)*math.Sin(t)
		// 画点 (考虑线宽)
		for w := -width / 2; w <= width/2; w++ {
			// 简单的线宽处理，可能不完美
			img.Set(int(x), int(y), c)
			// 稍微向圆心内外扩散以模拟线宽
			img.Set(int(float64(cx)+(float64(r)+float64(w))*math.Cos(t)), int(float64(cy)+(float64(r)+float64(w))*math.Sin(t)), c)
		}
	}
}

func drawEllipse(img draw.Image, op Operation) {
	// 修正：前端传来的 x, y 似乎是包围盒的左上角，而不是圆心
	// 因此圆心需要加上半径
	cx, cy := round(op.X+op.RadiusX), round(op.Y+op.RadiusY)
	rx, ry := round(op.RadiusX), round(op.RadiusY)
	strokeWidth := round(op.StrokeWidth)

	col, _ := parseColor(op.Stroke)
	col = applyOpacity(col, op.Opacity)

	// 绘制椭圆轮廓
	// 增加采样密度以获得更平滑的圆
	step := 1.0 / math.Max(float64(rx), float64(ry))

	for t := 0.0; t < 2*math.Pi; t += step {
		x := cx + int(float64(rx)*math.Cos(t))
		y := cy + int(float64(ry)*math.Sin(t))
		// 绘制点（考虑线宽）
		draw.Draw(img, image.Rect(x-strokeWidth/2, y-strokeWidth/2, x+strokeWidth/2+1, y+strokeWidth/2+1), image.NewUniform(col), image.Point{}, draw.Over)
	}
}

func drawLine(img draw.Image, op Operation) {
	if len(op.Points) < 4 {
		return
	}
	col, _ := parseColor(op.Stroke)
	col = applyOpacity(col, op.Opacity)
	width := round(op.StrokeWidth)

	x1, y1 := op.X+op.Points[0], op.Y+op.Points[1]
	x2, y2 := op.X+op.Points[2], op.Y+op.Points[3]
	drawThickLine(img, round(x1), round(y1), round(x2), round(y2), width, col)
}

func drawPen(img draw.Image, op Operation) {
	if len(op.Points) < 4 {
		return
	}
	col, _ := parseColor(op.Stroke)
	col = applyOpacity(col, op.Opacity)
	width := round(op.StrokeWidth)

	for i := 0; i < len(op.Points)-2; i += 2 {
		x1, y1 := op.Points[i], op.Points[i+1]
		x2, y2 := op.Points[i+2], op.Points[i+3]
		drawThickLine(img, round(x1), round(y1), round(x2), round(y2), width, col)
	}
}

func drawArrow(img draw.Image, op Operation) {
	if len(op.Points) < 4 {
		return
	}
	col, _ := parseColor(op.Stroke)
	col = applyOpacity(col, op.Opacity)
	width := round(op.StrokeWidth)

	x1, y1 := op.X+op.Points[0], op.Y+op.Points[1]
	x2, y2 := op.X+op.Points[2], op.Y+op.Points[3]

	// 计算角度
	angle := math.Atan2(y2-y1, x2-x1)

	// 箭头头部参数 (模拟 Konva 默认效果)
	// Konva 默认 pointerLength=10, pointerWidth=10
	// 这里我们根据线宽按比例缩放，或者设定一个最小值
	pointerLen := math.Max(10, float64(width)*3)
	pointerWidth := math.Max(10, float64(width)*3)

	// 调整终点，使其位于箭头底部中心，避免箭头尖端超出目标点太多
	// 或者 Konva 的行为是箭头尖端在 (x2, y2)
	// 假设尖端在 (x2, y2)

	// 计算箭头底部的中心点
	baseX := x2 - pointerLen*math.Cos(angle)
	baseY := y2 - pointerLen*math.Sin(angle)

	// 画主线 (从起点到箭头底部中心)
	drawThickLine(img, round(x1), round(y1), round(baseX), round(baseY), width, col)

	// 画实心三角形箭头
	// 计算三角形三个顶点
	// 顶点 1: 尖端 (x2, y2)
	p1x, p1y := x2, y2

	// 顶点 2 & 3: 底部两点
	// 垂直于主线的向量
	perpX := -math.Sin(angle)
	perpY := math.Cos(angle)

	p2x := baseX + perpX*(pointerWidth/2)
	p2y := baseY + perpY*(pointerWidth/2)

	p3x := baseX - perpX*(pointerWidth/2)
	p3y := baseY - perpY*(pointerWidth/2)

	// 填充三角形
	drawTriangle(img, round(p1x), round(p1y), round(p2x), round(p2y), round(p3x), round(p3y), col)
}

// 简单的扫描线算法填充三角形
func drawTriangle(img draw.Image, x1, y1, x2, y2, x3, y3 int, c color.Color) {
	// 按 y 排序顶点
	if y1 > y2 {
		x1, y1, x2, y2 = x2, y2, x1, y1
	}
	if y1 > y3 {
		x1, y1, x3, y3 = x3, y3, x1, y1
	}
	if y2 > y3 {
		x2, y2, x3, y3 = x3, y3, x2, y2
	}

	totalHeight := y3 - y1
	if totalHeight == 0 {
		return
	}

	for i := 0; i < totalHeight; i++ {
		secondHalf := i > y2-y1 || y2 == y1

		alpha := float64(i) / float64(totalHeight)

		// 计算 beta，注意防止除零
		var beta float64
		if secondHalf {
			if y3-y2 != 0 {
				beta = float64(i-(y2-y1)) / float64(y3-y2)
			}
		} else {
			if y2-y1 != 0 {
				beta = float64(i) / float64(y2-y1)
			}
		}

		ax := int(float64(x1) + float64(x3-x1)*alpha)
		bx := int(float64(x1) + float64(x2-x1)*beta)
		if secondHalf {
			bx = int(float64(x2) + float64(x3-x2)*beta)
		}

		if ax > bx {
			ax, bx = bx, ax
		}

		for j := ax; j <= bx; j++ {
			img.Set(j, y1+i, c)
		}
	}
}

func drawThickLine(img draw.Image, x0, y0, x1, y1, width int, col color.Color) {
	dx := float64(x1 - x0)
	dy := float64(y1 - y0)
	len := math.Sqrt(dx*dx + dy*dy)
	if len == 0 {
		return
	}
	udx := dx / len
	udy := dy / len
	pdx := -udy
	pdy := udx

	for i := 0; i < int(len); i++ {
		x := float64(x0) + udx*float64(i)
		y := float64(y0) + udy*float64(i)
		for w := -width / 2; w <= width/2; w++ {
			px := int(x + pdx*float64(w))
			py := int(y + pdy*float64(w))
			img.Set(px, py, col)
		}
	}
}

// ParseOperations 解析前端传来的 JSON 操作
func ParseOperations(input interface{}) ([]Operation, error) {
	var ops []Operation

	// 处理 map[string]interface{}
	if m, ok := input.(map[string]interface{}); ok {
		// 尝试从 "annotations" 字段获取
		if ann, ok := m["annotations"]; ok {
			if annMap, ok := ann.(map[string]interface{}); ok {
				for _, v := range annMap {
					if opMap, ok := v.(map[string]interface{}); ok {
						op := parseSingleOperation(opMap)
						ops = append(ops, op)
					}
				}
			}
		} else {
			// 直接是 map
			for _, v := range m {
				if opMap, ok := v.(map[string]interface{}); ok {
					op := parseSingleOperation(opMap)
					ops = append(ops, op)
				}
			}
		}
	} else if s, ok := input.([]interface{}); ok {
		// 处理 []interface{}
		for _, v := range s {
			if opMap, ok := v.(map[string]interface{}); ok {
				op := parseSingleOperation(opMap)
				ops = append(ops, op)
			}
		}
	}

	// 按 ID 排序保证稳定性
	sort.Slice(ops, func(i, j int) bool {
		return ops[i].ID < ops[j].ID
	})

	return ops, nil
}

func parseSingleOperation(m map[string]interface{}) Operation {
	op := Operation{}
	if v, ok := m["id"].(string); ok {
		op.ID = v
	}
	if v, ok := m["name"].(string); ok {
		op.Type = v
	}
	op.X = getFloat(m["x"])
	op.Y = getFloat(m["y"])
	op.Width = getFloat(m["width"])
	op.Height = getFloat(m["height"])
	if v, ok := m["stroke"].(string); ok {
		op.Stroke = v
	}
	op.StrokeWidth = getFloat(m["strokeWidth"])
	op.StrokeWidthOrig = getFloat(m["strokeWidth"])
	if v, ok := m["fill"].(string); ok {
		op.Fill = v
	}
	op.Opacity = getFloat(m["opacity"])
	if op.Opacity == 0 && m["opacity"] == nil {
		op.Opacity = 1 // 默认不透明
	}

	// Text
	if v, ok := m["text"].(string); ok {
		op.Text = v
	}
	if v, ok := m["fontFamily"].(string); ok {
		op.FontFamily = v
	}
	op.FontSize = getFloat(m["fontSize"])
	if v, ok := m["align"].(string); ok {
		op.Align = v
	}
	if v, ok := m["fontStyle"].(string); ok {
		op.FontStyle = v
	}

	// Rect
	op.CornerRadius = getFloat(m["cornerRadius"])

	// Ellipse
	op.RadiusX = getFloat(m["radiusX"])
	op.RadiusY = getFloat(m["radiusY"])

	// Points
	if pts, ok := m["points"].([]interface{}); ok {
		for _, p := range pts {
			op.Points = append(op.Points, getFloat(p))
		}
	} else if pts, ok := m["points"].([]float64); ok {
		op.Points = pts
	}

	if v, ok := m["lineCap"].(string); ok {
		op.LineCap = v
	}

	// Scale
	op.ScaleX = 1.0
	op.ScaleY = 1.0
	if val, ok := m["scaleX"]; ok {
		op.ScaleX = getFloat(val)
	}
	if val, ok := m["scaleY"]; ok {
		op.ScaleY = getFloat(val)
	}

	return op
}

func getFloat(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	}
	return 0
}

// ImageTask 图片处理任务
type ImageTask struct {
	ID        string // 图片ID（用于追踪）
	ImagePath string // 图片路径
}

// ImageResult 图片处理结果
type ImageResult struct {
	ID         string // 图片ID
	ImagePath  string // 原始图片路径
	OutputPath string // 处理后的图片路径（成功时）
	Success    bool   // 是否成功
	ErrorMsg   string // 错误信息（失败时）
}

// BatchImageResult 批量处理结果汇总
type BatchImageResult struct {
	Total        int           // 总数
	SuccessCount int           // 成功数量
	FailCount    int           // 失败数量
	Results      []ImageResult // 详细结果
}

// ApplyImageOperations 批量处理图片
func ApplyImageOperations(
	tasks []ImageTask,
	operationsInput interface{},
	refWidth, refHeight, scaledBy float64,
	outputDir string,
	logger *zap.SugaredLogger,
) (*BatchImageResult, error) {
	ops, err := ParseOperations(operationsInput)
	if err != nil {
		return nil, fmt.Errorf("解析操作失败: %w", err)
	}

	if scaledBy <= 0 {
		scaledBy = 1.0
	}

	// 使用通用并发函数处理所有图片
	concurrentResults := ExecuteConcurrent(len(tasks), 10, func(taskIndex int) (interface{}, error) {
		task := tasks[taskIndex]
		outPath, err := processSingleImage(task.ImagePath, ops, refWidth, refHeight, outputDir, logger)

		// 返回详细结果，即使失败也返回结果对象（不返回 error）
		result := ImageResult{
			ID:        task.ID,
			ImagePath: task.ImagePath,
		}

		if err != nil {
			result.Success = false
			result.ErrorMsg = err.Error()
			logger.Warnw("处理图片失败", "imageID", task.ID, "path", task.ImagePath, "err", err)
		} else {
			result.Success = true
			result.OutputPath = outPath
		}

		return result, nil // 始终返回 nil error，让处理继续
	})

	// 收集结果
	batchResult := &BatchImageResult{
		Total:   len(tasks),
		Results: make([]ImageResult, 0, len(tasks)),
	}

	for _, r := range concurrentResults {
		if r.Error != nil {
			// 理论上不会到这里，因为我们总是返回 nil error
			logger.Errorw("意外错误", "err", r.Error)
			continue
		}

		result := r.Data.(ImageResult)
		batchResult.Results = append(batchResult.Results, result)

		if result.Success {
			batchResult.SuccessCount++
		} else {
			batchResult.FailCount++
		}
	}

	return batchResult, nil
}

// processSingleImage 处理单张图片
func processSingleImage(imgPath string, ops []Operation, refWidth, refHeight float64, outputDir string, logger *zap.SugaredLogger) (string, error) {
	// 加载图片
	srcImg, err := imaging.Open(imgPath)
	if err != nil {
		logger.Errorw("打开图片失败", "path", imgPath, "err", err)
		return "", err
	}

	bounds := srcImg.Bounds()
	targetWidth := float64(bounds.Dx())
	targetHeight := float64(bounds.Dy())

	// 计算缩放比例
	// 核心逻辑说明：
	// 1. 前端传来的 shownImageDimensions (refWidth, refHeight) 是图片在前端编辑器中的显示尺寸（CSS像素）。
	// 2. annotations 中的所有坐标 (x, y) 和尺寸 (width, height) 都是基于这个显示尺寸的坐标系。
	// 3. 即使前端进行了缩放 (scaledBy 变化)，refWidth 和 annotations 也会同步变化（或者说它们处于同一个相对坐标系中）。
	// 4. 因此，后端只需要计算 "后端实际图片尺寸 / 前端显示尺寸" 得到全局缩放比例 (scaleX, scaleY)。
	// 5. 元素的实际显示大小由 width * op.ScaleX 决定（Konva 的行为），因此在映射时需要同时应用元素缩放和全局缩放。
	scaleX := 1.0
	scaleY := 1.0
	if refWidth > 0 && refHeight > 0 {
		scaleX = targetWidth / refWidth
		scaleY = targetHeight / refHeight
	}

	// 创建绘图上下文
	dstImg := image.NewRGBA(bounds)
	draw.Draw(dstImg, bounds, srcImg, bounds.Min, draw.Src)

	// 应用操作
	for _, op := range ops {
		// 1. 先应用元素自身的缩放 (ScaleX, ScaleY)
		// 注意：位置 X, Y 通常不受自身缩放影响（除非是中心缩放，但 Konva 默认左上角）
		// 只有尺寸相关的属性需要应用自身缩放
		elemScaleX := op.ScaleX
		elemScaleY := op.ScaleY
		if elemScaleX == 0 {
			elemScaleX = 1
		}
		if elemScaleY == 0 {
			elemScaleY = 1
		}

		effectiveWidth := op.Width * elemScaleX
		effectiveHeight := op.Height * elemScaleY
		effectiveRadiusX := op.RadiusX * elemScaleX
		effectiveRadiusY := op.RadiusY * elemScaleY

		// 2. 再应用全局缩放 (scaleX, scaleY) 映射到后端图片尺寸
		scaledOp := op
		scaledOp.X *= scaleX
		scaledOp.Y *= scaleY
		scaledOp.Width = effectiveWidth * scaleX
		scaledOp.Height = effectiveHeight * scaleY
		scaledOp.RadiusX = effectiveRadiusX * scaleX
		scaledOp.RadiusY = effectiveRadiusY * scaleY

		// 字体和描边按平均比例缩放
		// 字体大小通常不受 ScaleX/Y 影响（除非是整体缩放），这里假设 ScaleX/Y 也会影响字体
		// 但通常 Text 的 ScaleX/Y 是 1。如果 Text 被缩放了，fontSize 视觉上也会变。
		avgGlobalScale := (scaleX + scaleY) / 2
		avgElemScale := (elemScaleX + elemScaleY) / 2

		scaledOp.FontSize *= avgGlobalScale * avgElemScale
		scaledOp.StrokeWidth *= avgGlobalScale * avgElemScale // 描边宽度也随元素缩放
		scaledOp.CornerRadius *= avgGlobalScale * avgElemScale
		scaledOp.GlobalScale = avgGlobalScale

		//fmt.Println("**********************")
		//fmt.Println(scaleX, scaleY, elemScaleX, elemScaleY, scaledOp.StrokeWidth)

		// 缩放点坐标
		if len(op.Points) > 0 {
			newPoints := make([]float64, len(op.Points))
			for i := 0; i < len(op.Points)-1; i += 2 {
				// 点坐标是绝对位置，只受全局缩放影响
				// 除非 Points 是相对于 (X,Y) 的，但这里看起来是绝对坐标
				newPoints[i] = op.Points[i] * scaleX
				newPoints[i+1] = op.Points[i+1] * scaleY
			}
			scaledOp.Points = newPoints
		}

		//opsJSON, _ := json.Marshal(scaledOp)
		//fmt.Println("opsJSON", string(opsJSON))

		switch op.Type {
		case "Text":
			drawText(dstImg, scaledOp)
		case "Rect":
			drawRect(dstImg, scaledOp)
		case "Ellipse":
			drawEllipse(dstImg, scaledOp)
		case "Line":
			drawLine(dstImg, scaledOp)
		case "Pen":
			drawPen(dstImg, scaledOp)
		case "Arrow":
			drawArrow(dstImg, scaledOp)
		}
	}

	// 保存图片
	outPath := imgPath
	if outputDir != "" {
		base := filepath.Base(imgPath)
		ext := filepath.Ext(base)
		name := strings.TrimSuffix(base, ext)
		outPath = filepath.Join(outputDir, name+"_processed"+ext)
	}

	if err := imaging.Save(dstImg, outPath); err != nil {
		logger.Errorw("保存图片失败", "path", outPath, "err", err)
		return "", err
	}
	return outPath, nil
}

// processSingleImageNoScale 基于原图尺寸直接绘制，不进行任何缩放
func processSingleImageNoScale(imgPath string, ops []Operation, outputDir string, logger *zap.SugaredLogger) (string, error) {
    srcImg, err := imaging.Open(imgPath)
    if err != nil {
        logger.Errorw("打开图片失败", "path", imgPath, "err", err)
        return "", err
    }

    bounds := srcImg.Bounds()
    dstImg := image.NewRGBA(bounds)
    draw.Draw(dstImg, bounds, srcImg, bounds.Min, draw.Src)

    for _, op := range ops {
        scaledOp := op
        // 保持原始尺寸与坐标，不做缩放
        scaledOp.StrokeWidthOrig = op.StrokeWidth
        scaledOp.GlobalScale = 1
        // Points 保持不变
        switch strings.ToLower(scaledOp.Type) {
        case "text":
            drawText(dstImg, scaledOp)
        case "rect":
            drawRect(dstImg, scaledOp)
        case "ellipse":
            drawEllipse(dstImg, scaledOp)
        case "line":
            drawLine(dstImg, scaledOp)
        case "pen":
            drawPen(dstImg, scaledOp)
        case "arrow":
            drawArrow(dstImg, scaledOp)
        case "mosaic":
            applyMosaic(dstImg, round(scaledOp.X), round(scaledOp.Y), round(scaledOp.Width), round(scaledOp.Height), int(math.Max(1, scaledOp.StrokeWidth)))
        }
    }

    outPath := imgPath
    if outputDir != "" {
        base := filepath.Base(imgPath)
        ext := filepath.Ext(base)
        name := strings.TrimSuffix(base, ext)
        outPath = filepath.Join(outputDir, name+"_processed"+ext)
    }

    if err := imaging.Save(dstImg, outPath); err != nil {
        logger.Errorw("保存图片失败", "path", outPath, "err", err)
        return "", err
    }
    return outPath, nil
}

// ApplyImageOperationsSingleNoScale 单图处理入口（新编辑器，无缩放）
func ApplyImageOperationsSingleNoScale(
    imagePath string,
    flow *entity.NewEditorImageFlow,
    outputDir string,
    logger *zap.SugaredLogger,
) (string, error) {
    ops, err := BuildOperationsFromNewFlowLoose(flow)
    if err != nil {
        return "", fmt.Errorf("构建操作失败: %w", err)
    }
    return processSingleImageNoScale(imagePath, ops, outputDir, logger)
}

func processSingleImageUniform(imgPath string, ops []Operation, baseWidth, baseHeight float64, outputDir string, logger *zap.SugaredLogger) (string, error) {
	srcImg, err := imaging.Open(imgPath)
	if err != nil {
		logger.Errorw("打开图片失败", "path", imgPath, "err", err)
		return "", err
	}

	bounds := srcImg.Bounds()
	targetWidth := float64(bounds.Dx())
	targetHeight := float64(bounds.Dy())

	if baseWidth <= 0 || baseHeight <= 0 {
		return "", fmt.Errorf("基准尺寸无效")
	}

	kx := targetWidth / baseWidth
	ky := targetHeight / baseHeight
	k := kx
	if ky < k {
		k = ky
	}

	dstImg := image.NewRGBA(bounds)
	draw.Draw(dstImg, bounds, srcImg, bounds.Min, draw.Src)

	for _, op := range ops {
		scaledOp := op
		scaledOp.X *= k
		scaledOp.Y *= k
		scaledOp.Width *= k
		scaledOp.Height *= k
		scaledOp.RadiusX *= k
		scaledOp.RadiusY *= k
		scaledOp.FontSize *= k
		scaledOp.StrokeWidthOrig = op.StrokeWidth
		scaledOp.StrokeWidth = op.StrokeWidth * k
		scaledOp.GlobalScale = k

		if len(op.Points) > 0 {
			newPoints := make([]float64, len(op.Points))
			for i := 0; i < len(op.Points)-1; i += 2 {
				newPoints[i] = op.Points[i] * k
				newPoints[i+1] = op.Points[i+1] * k
			}
			scaledOp.Points = newPoints
		}

		switch scaledOp.Type {
		case "Text":
			drawText(dstImg, scaledOp)
		case "Rect":
			drawRect(dstImg, scaledOp)
		case "Ellipse":
			drawEllipse(dstImg, scaledOp)
		case "Line":
			drawLine(dstImg, scaledOp)
		case "Pen":
			drawPen(dstImg, scaledOp)
		case "Arrow":
			drawArrow(dstImg, scaledOp)
		case "Mosaic":
			applyMosaic(dstImg, round(scaledOp.X), round(scaledOp.Y), round(scaledOp.Width), round(scaledOp.Height), int(math.Max(1, scaledOp.StrokeWidth)))
		}
	}

	outPath := imgPath
	if outputDir != "" {
		base := filepath.Base(imgPath)
		ext := filepath.Ext(base)
		name := strings.TrimSuffix(base, ext)
		outPath = filepath.Join(outputDir, name+"_processed"+ext)
	}

	if err := imaging.Save(dstImg, outPath); err != nil {
		logger.Errorw("保存图片失败", "path", outPath, "err", err)
		return "", err
	}
	return outPath, nil
}

func applyMosaic(img draw.Image, x, y, w, h, size int) {
	if size <= 0 {
		size = 10
	}
	x2 := x + w
	y2 := y + h
	for yy := y; yy < y2; yy += size {
		for xx := x; xx < x2; xx += size {
			// 采样左上角像素颜色
			c := img.At(xx, yy)
			// 填充一个 size x size 的方块
			for py := yy; py < yy+size && py < y2; py++ {
				for px := xx; px < xx+size && px < x2; px++ {
					img.Set(px, py, c)
				}
			}
		}
	}
}

func BuildOperationsFromNewFlow(flow *entity.NewEditorImageFlow) ([]Operation, float64, float64, error) {
	if flow == nil {
		return nil, 0, 0, fmt.Errorf("flow 为空")
	}
	baseW := flow.Image.Width
	baseH := flow.Image.Height
	if baseW <= 0 || baseH <= 0 {
		return nil, 0, 0, fmt.Errorf("flow.image 宽高无效")
	}

	ops := make([]Operation, 0, len(flow.Texts)+len(flow.Shapes))

	for _, t := range flow.Texts {
		ops = append(ops, Operation{
			ID:          t.ID,
			Type:        "Text",
			X:           t.X,
			Y:           t.Y,
			Width:       0,
			Height:      0,
			Fill:        t.Color,
			Stroke:      "transparent",
			FontSize:    t.FontSize,
			Text:        t.Text,
			Opacity:     1,
			ScaleX:      1,
			ScaleY:      1,
			GlobalScale: 1,
		})
	}

	for _, s := range flow.Shapes {
		typ := strings.ToLower(s.Type)
		switch typ {
		case "rect":
			fillColor := "transparent"
			if strings.ToLower(s.Fill) == "fill" {
				fillColor = s.Color
			}
			ops = append(ops, Operation{
				ID:              s.ID,
				Type:            "Rect",
				X:               s.X,
				Y:               s.Y,
				Width:           s.W,
				Height:          s.H,
				Fill:            fillColor,
				Stroke:          s.Color,
				StrokeWidth:     s.LineWidth,
				StrokeWidthOrig: s.LineWidth,
				Opacity:         1,
				ScaleX:          1,
				ScaleY:          1,
			})
		case "circle":
			fillColor := "transparent"
			if strings.ToLower(s.Fill) == "fill" {
				fillColor = s.Color
			}
			ops = append(ops, Operation{
				ID:              s.ID,
				Type:            "Ellipse",
				X:               s.X,
				Y:               s.Y,
				RadiusX:         s.W / 2,
				RadiusY:         s.H / 2,
				Fill:            fillColor,
				Stroke:          s.Color,
				StrokeWidth:     s.LineWidth,
				StrokeWidthOrig: s.LineWidth,
				Opacity:         1,
				ScaleX:          1,
				ScaleY:          1,
			})
		case "mosaic":
			ops = append(ops, Operation{
				ID:          s.ID,
				Type:        "Mosaic",
				X:           s.X,
				Y:           s.Y,
				Width:       s.W,
				Height:      s.H,
				StrokeWidth: float64(s.MosaicSize),
				Opacity:     1,
			})
		}
	}

	// 稳定排序
	sort.Slice(ops, func(i, j int) bool { return ops[i].ID < ops[j].ID })
	return ops, baseW, baseH, nil
}

// BuildOperationsFromNewFlowLoose 构建操作，不强制要求 image 宽高
func BuildOperationsFromNewFlowLoose(flow *entity.NewEditorImageFlow) ([]Operation, error) {
    if flow == nil {
        return nil, fmt.Errorf("flow 为空")
    }

    ops := make([]Operation, 0, len(flow.Texts)+len(flow.Shapes))

    for _, t := range flow.Texts {
        ops = append(ops, Operation{
            ID:         t.ID,
            Type:       "Text",
            X:          t.X,
            Y:          t.Y,
            Fill:       t.Color,
            Stroke:     "transparent",
            FontSize:   t.FontSize,
            Text:       t.Text,
            Opacity:    1,
            ScaleX:     1,
            ScaleY:     1,
            GlobalScale: 1,
        })
    }

    for _, s := range flow.Shapes {
        typ := strings.ToLower(s.Type)
        switch typ {
        case "rect":
            fillColor := "transparent"
            if strings.ToLower(s.Fill) == "fill" {
                fillColor = s.Color
            }
            ops = append(ops, Operation{
                ID:            s.ID,
                Type:          "Rect",
                X:             s.X,
                Y:             s.Y,
                Width:         s.W,
                Height:        s.H,
                Fill:          fillColor,
                Stroke:        s.Color,
                StrokeWidth:   s.LineWidth,
                StrokeWidthOrig: s.LineWidth,
                Opacity:       1,
                ScaleX:        1,
                ScaleY:        1,
            })
        case "circle":
            fillColor := "transparent"
            if strings.ToLower(s.Fill) == "fill" {
                fillColor = s.Color
            }
            ops = append(ops, Operation{
                ID:            s.ID,
                Type:          "Ellipse",
                X:             s.X,
                Y:             s.Y,
                RadiusX:       s.W / 2,
                RadiusY:       s.H / 2,
                Fill:          fillColor,
                Stroke:        s.Color,
                StrokeWidth:   s.LineWidth,
                StrokeWidthOrig: s.LineWidth,
                Opacity:       1,
                ScaleX:        1,
                ScaleY:        1,
            })
        case "mosaic":
            ops = append(ops, Operation{
                ID:          s.ID,
                Type:        "Mosaic",
                X:           s.X,
                Y:           s.Y,
                Width:       s.W,
                Height:      s.H,
                StrokeWidth: float64(s.MosaicSize),
                Opacity:     1,
            })
        }
    }

    sort.Slice(ops, func(i, j int) bool { return ops[i].ID < ops[j].ID })
    return ops, nil
}

func ApplyImageOperationsV2(
	tasks []ImageTask,
	flow *entity.NewEditorImageFlow,
	outputDir string,
	logger *zap.SugaredLogger,
) (*BatchImageResult, error) {
	ops, baseW, baseH, err := BuildOperationsFromNewFlow(flow)
	if err != nil {
		return nil, fmt.Errorf("构建操作失败: %w", err)
	}

	concurrentResults := ExecuteConcurrent(len(tasks), 10, func(taskIndex int) (interface{}, error) {
		task := tasks[taskIndex]
		outPath, err := processSingleImageUniform(task.ImagePath, ops, baseW, baseH, outputDir, logger)
		result := ImageResult{ID: task.ID, ImagePath: task.ImagePath}
		if err != nil {
			result.Success = false
			result.ErrorMsg = err.Error()
			logger.Warnw("处理图片失败", "imageID", task.ID, "path", task.ImagePath, "err", err)
		} else {
			result.Success = true
			result.OutputPath = outPath
		}
		return result, nil
	})

	batchResult := &BatchImageResult{Total: len(tasks), Results: make([]ImageResult, 0, len(tasks))}
	for _, r := range concurrentResults {
		if r.Error != nil {
			logger.Errorw("意外错误", "err", r.Error)
			continue
		}
		res := r.Data.(ImageResult)
		batchResult.Results = append(batchResult.Results, res)
		if res.Success {
			batchResult.SuccessCount++
		} else {
			batchResult.FailCount++
		}
	}
	return batchResult, nil
}
