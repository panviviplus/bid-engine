package utils

import (
	"fmt"
	"image"
	"image/draw"
	"math"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"
	"go.uber.org/zap"
)

type MaterialImageEditFlowImage struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type MaterialImageEditFlowText struct {
	ID       string  `json:"id"`
	Text     string  `json:"text"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Color    string  `json:"color"`
	FontSize float64 `json:"fontSize"`
}

type MaterialImageEditFlowShape struct {
	ID         string  `json:"id"`
	Type       string  `json:"type"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	W          float64 `json:"w"`
	H          float64 `json:"h"`
	Color      string  `json:"color"`
	LineWidth  float64 `json:"lineWidth"`
	Fill       string  `json:"fill"`
	MosaicSize int     `json:"mosaicSize"`
}

type MaterialImageEditFlow struct {
	Image  *MaterialImageEditFlowImage  `json:"image"`
	Texts  []MaterialImageEditFlowText  `json:"texts"`
	Shapes []MaterialImageEditFlowShape `json:"shapes"`
}

func (f *MaterialImageEditFlow) ScaleToTarget(targetW, targetH float64) *MaterialImageEditFlow {
	if f == nil || f.Image == nil || f.Image.Width <= 0 || f.Image.Height <= 0 || targetW <= 0 || targetH <= 0 {
		return f
	}
	scaleX := targetW / f.Image.Width
	scaleY := targetH / f.Image.Height
	scale := math.Min(scaleX, scaleY)
	if scale <= 0 {
		return f
	}
	out := &MaterialImageEditFlow{
		Image: &MaterialImageEditFlowImage{Width: targetW, Height: targetH},
		Texts: make([]MaterialImageEditFlowText, 0, len(f.Texts)),
		Shapes: make([]MaterialImageEditFlowShape, 0, len(f.Shapes)),
	}
	for _, t := range f.Texts {
		nt := t
		nt.X *= scale
		nt.Y *= scale
		nt.FontSize *= scale
		out.Texts = append(out.Texts, nt)
	}
	for _, s := range f.Shapes {
		ns := s
		ns.X *= scale
		ns.Y *= scale
		ns.W *= scale
		ns.H *= scale
		ns.LineWidth *= scale
		if ns.MosaicSize > 0 {
			ns.MosaicSize = int(math.Round(float64(ns.MosaicSize) * scale))
			if ns.MosaicSize < 1 {
				ns.MosaicSize = 1
			}
		}
		out.Shapes = append(out.Shapes, ns)
	}
	return out
}

func ApplyMaterialImageEditFlow(srcPath string, flow *MaterialImageEditFlow, outputDir string, logger *zap.SugaredLogger) (string, error) {
	if strings.TrimSpace(srcPath) == "" {
		return "", fmt.Errorf("invalid srcPath")
	}
	// 如果 flow 无效，直接返回原图路径
	if flow == nil || flow.Image == nil || flow.Image.Width <= 0 || flow.Image.Height <= 0 {
		return srcPath, nil
	}
	srcImg, err := imaging.Open(srcPath)
	if err != nil {
		if logger != nil {
			logger.Errorw("打开图片失败", "path", srcPath, "err", err)
		}
		return "", err
	}
	bounds := srcImg.Bounds()
	dstImg := image.NewRGBA(bounds)
	draw.Draw(dstImg, bounds, srcImg, bounds.Min, draw.Src)

	for _, t := range flow.Texts {
		op := Operation{
			ID:       t.ID,
			Type:     "Text",
			X:        t.X,
			Y:        t.Y,
			Text:     t.Text,
			Fill:     t.Color,
			FontSize: t.FontSize,
			Opacity:  1,
			ScaleX:   1,
			ScaleY:   1,
		}
		drawText(dstImg, op)
	}
	for _, s := range flow.Shapes {
		typ := strings.ToLower(strings.TrimSpace(s.Type))
		switch typ {
		case "rect":
			fill := "transparent"
			stroke := "transparent"
			strokeWidth := s.LineWidth
			if strings.ToLower(s.Fill) == "fill" {
				fill = s.Color
				strokeWidth = 0
			} else {
				stroke = s.Color
			}
			op := Operation{
				ID:              s.ID,
				Type:            "Rect",
				X:               s.X,
				Y:               s.Y,
				Width:           s.W,
				Height:          s.H,
				Fill:            fill,
				Stroke:          stroke,
				StrokeWidth:     strokeWidth,
				StrokeWidthOrig: strokeWidth,
				Opacity:         1,
				ScaleX:          1,
				ScaleY:          1,
				GlobalScale:     1,
			}
			drawRect(dstImg, op)
		case "circle":
			stroke := s.Color
			strokeWidth := s.LineWidth
			op := Operation{
				ID:              s.ID,
				Type:            "Ellipse",
				X:               s.X,
				Y:               s.Y,
				RadiusX:         s.W / 2,
				RadiusY:         s.H / 2,
				Stroke:          stroke,
				StrokeWidth:     strokeWidth,
				StrokeWidthOrig: strokeWidth,
				Opacity:         1,
				ScaleX:          1,
				ScaleY:          1,
				GlobalScale:     1,
			}
			drawEllipse(dstImg, op)
		case "mosaic":
			size := s.MosaicSize
			if size <= 0 {
				size = int(math.Max(1, s.LineWidth))
			}
			applyMosaic(dstImg, round(s.X), round(s.Y), round(s.W), round(s.H), size)
		}
	}

	outPath := srcPath
	if outputDir != "" {
		base := filepath.Base(srcPath)
		ext := filepath.Ext(base)
		name := strings.TrimSuffix(base, ext)
		outPath = filepath.Join(outputDir, name+"_processed"+ext)
	}
	if err := imaging.Save(dstImg, outPath); err != nil {
		if logger != nil {
			logger.Errorw("保存图片失败", "path", outPath, "err", err)
		}
		return "", err
	}
	return outPath, nil
}

