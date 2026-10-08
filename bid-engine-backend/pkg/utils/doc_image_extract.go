package utils

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/richardlehane/mscfb"
)

type DocImage struct {
	Content []byte
	Ext     string
}

func DocExtractImagesAuto(path string) ([]DocImage, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".docx":
		return extractImagesFromDocx(path)
	case ".doc":
		return extractImagesFromDocOLE2(path)
	case ".pdf":
		return extractImagesFromPDFSimple(path)
	default:
		return nil, fmt.Errorf("不支持的文件类型: %s", ext)
	}
}

func extractImagesFromDocx(docxPath string) ([]DocImage, error) {
	reader, err := zip.OpenReader(docxPath)
	if err != nil {
		return nil, fmt.Errorf("打开 docx 文件失败: %w", err)
	}
	defer reader.Close()

	var images []DocImage
	seen := make(map[string]bool)

	for _, file := range reader.File {
		if !strings.HasPrefix(file.Name, "word/media/") {
			continue
		}
		if file.FileInfo().IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(file.Name))
		if !docIsImageExtension(ext) {
			continue
		}
		fr, err := file.Open()
		if err != nil {
			continue
		}
		content, err := io.ReadAll(fr)
		fr.Close()
		if err != nil {
			continue
		}
		h := crc32Key(content)
		if !seen[h] {
			seen[h] = true
			images = append(images, DocImage{Content: content, Ext: ext})
		}
	}

	for _, file := range reader.File {
		if !strings.HasPrefix(file.Name, "word/embeddings/") {
			continue
		}
		if file.FileInfo().IsDir() {
			continue
		}
		fr, err := file.Open()
		if err != nil {
			continue
		}
		content, err := io.ReadAll(fr)
		fr.Close()
		if err != nil || len(content) == 0 {
			continue
		}
		oleImages := extractFromOLE2Data(content)
		for _, im := range oleImages {
			h := crc32Key(im.Content)
			if !seen[h] {
				seen[h] = true
				images = append(images, im)
			}
		}
	}

	return images, nil
}

func extractFromOLE2Data(data []byte) []DocImage {
	var out []DocImage
	r := bytes.NewReader(data)
	doc, err := mscfb.New(r)
	if err != nil {
		return out
	}

	seen := make(map[string]bool)
	for _, err := doc.Next(); err == nil; _, err = doc.Next() {
		streamData, err := io.ReadAll(doc)
		if err != nil || len(streamData) == 0 {
			continue
		}

		if direct := extractDirectImagesStrict(streamData); len(direct) > 0 {
			for _, im := range direct {
				h := crc32Key(im.Content)
				if !seen[h] {
					seen[h] = true
					out = append(out, im)
				}
			}
		}

		wrapped := extractBLIPImages(streamData)
		for _, im := range wrapped {
			h := crc32Key(im.Content)
			if !seen[h] {
				seen[h] = true
				out = append(out, im)
			}
		}
	}

	return out
}

func docIsImageExtension(ext string) bool {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff", ".tif", ".webp":
		return true
	default:
		return false
	}
}

func extractImagesFromDocOLE2(docPath string) ([]DocImage, error) {
	f, err := os.Open(docPath)
	if err != nil {
		return nil, fmt.Errorf("打开DOC失败: %w", err)
	}
	defer f.Close()

	doc, err := mscfb.New(f)
	if err != nil {
		return nil, fmt.Errorf("解析OLE2结构失败: %w", err)
	}

	var images []DocImage
	seen := make(map[string]bool)
	for _, err := doc.Next(); err == nil; _, err = doc.Next() {
		data, err := io.ReadAll(doc)
		if err != nil || len(data) == 0 {
			continue
		}
		if direct := extractDirectImagesStrict(data); len(direct) > 0 {
			for _, im := range direct {
				h := crc32Key(im.Content)
				if !seen[h] {
					seen[h] = true
					images = append(images, im)
				}
			}
		}
		wrapped := extractBLIPImages(data)
		for _, im := range wrapped {
			h := crc32Key(im.Content)
			if !seen[h] {
				seen[h] = true
				images = append(images, im)
			}
		}
	}
	return images, nil
}

func extractDirectImagesStrict(data []byte) []DocImage {
	var out []DocImage
	if len(data) >= 4 && data[0] == 0xFF && data[1] == 0xD8 {
		end := bytes.LastIndex(data, []byte{0xFF, 0xD9})
		if end >= 2 {
			img := data[:end+2]
			if len(img) == len(data) && isValidJPEG(img) && decodeValidate(".jpg", img) {
				out = append(out, DocImage{Content: append([]byte(nil), img...), Ext: ".jpg"})
				return out
			}
		}
	}
	if len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
		j := 8
		for j+12 <= len(data) {
			if string(data[j+4:j+8]) == "IEND" {
				ln := int(binary.BigEndian.Uint32(data[j : j+4]))
				end := j + 8 + ln + 4
				if end == len(data) {
					img := data[:end]
					if isValidPNG(img) && decodeValidate(".png", img) {
						out = append(out, DocImage{Content: append([]byte(nil), img...), Ext: ".png"})
						return out
					}
				}
				break
			}
			if j+8 > len(data) {
				break
			}
			ln := int(binary.BigEndian.Uint32(data[j : j+4]))
			j += 8 + ln + 4
		}
	}
	if len(data) >= 6 && (bytes.Equal(data[:6], []byte("GIF87a")) || bytes.Equal(data[:6], []byte("GIF89a"))) {
		k := 6
		for k < len(data) {
			if data[k] == 0x3B {
				k++
				if k == len(data) && decodeValidate(".gif", data) {
					out = append(out, DocImage{Content: append([]byte(nil), data...), Ext: ".gif"})
					return out
				}
				break
			}
			k++
		}
	}
	if len(data) >= 6 && data[0] == 'B' && data[1] == 'M' {
		size := int(binary.LittleEndian.Uint32(data[2:6]))
		if size == len(data) && isValidBMP(data) {
			out = append(out, DocImage{Content: append([]byte(nil), data...), Ext: ".bmp"})
			return out
		}
	}
	return out
}

func extractBLIPImages(data []byte) []DocImage {
	var out []DocImage
	seen := make(map[string]bool)
	out = append(out, parseOfficeArtBlips(data, seen)...)
	out = append(out, extractFromWMFEMF(data, seen)...)
	for i := 0; i < len(data); i++ {
		if i+2 < len(data) && data[i] == 0xFF && data[i+1] == 0xD8 {
			end := i + 2
			for end+1 < len(data) {
				if data[end] == 0xFF && data[end+1] == 0xD9 {
					end += 2
					img := data[i:end]
					if isValidJPEG(img) && decodeValidate(".jpg", img) {
						h := crc32Key(img)
						if !seen[h] {
							seen[h] = true
							out = append(out, DocImage{Content: append([]byte(nil), img...), Ext: ".jpg"})
						}
					}
					i = end - 1
					break
				}
				end++
			}
		}
		if i+8 <= len(data) && bytes.Equal(data[i:i+8], []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
			end := i + 8
			for end+12 <= len(data) {
				if string(data[end+4:end+8]) == "IEND" {
					ln := int(binary.BigEndian.Uint32(data[end : end+4]))
					end = end + 8 + ln + 4
					if end <= len(data) {
						img := data[i:end]
						if isValidPNG(img) && decodeValidate(".png", img) {
							h := crc32Key(img)
							if !seen[h] {
								seen[h] = true
								out = append(out, DocImage{Content: append([]byte(nil), img...), Ext: ".png"})
							}
						}
						i = end - 1
					}
					break
				}
				if end+8 > len(data) {
					break
				}
				ln := int(binary.BigEndian.Uint32(data[end : end+4]))
				end += 8 + ln + 4
			}
		}
	}
	return out
}

func parseOfficeArtBlips(b []byte, seen map[string]bool) []DocImage {
	var out []DocImage
	if len(b) < 12 {
		return out
	}
	validTypes := map[uint16]bool{0xF01D: true, 0xF01E: true, 0xF01F: true, 0xF01A: true, 0xF01B: true}
	for i := 0; i+8 <= len(b); i++ {
		recType := binary.LittleEndian.Uint16(b[i+2 : i+4])
		recLen := int(binary.LittleEndian.Uint32(b[i+4 : i+8]))
		if !validTypes[recType] {
			continue
		}
		payloadStart := i + 8
		payloadEnd := payloadStart + recLen
		if recLen <= 0 || recLen > 64*1024*1024 || payloadEnd > len(b) {
			continue
		}
		payload := b[payloadStart:payloadEnd]
		switch recType {
		case 0xF01D:
			if len(payload) >= 4 && payload[0] == 0xFF && payload[1] == 0xD8 {
				end := bytes.LastIndex(payload, []byte{0xFF, 0xD9})
				if end >= 2 {
					img := payload[:end+2]
					if isValidJPEG(img) && decodeValidate(".jpg", img) {
						h := crc32Key(img)
						if !seen[h] {
							seen[h] = true
							out = append(out, DocImage{Content: append([]byte(nil), img...), Ext: ".jpg"})
						}
					}
				}
			}
		case 0xF01E:
			if len(payload) >= 8 && bytes.Equal(payload[:8], []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
				j := 8
				for j+12 <= len(payload) {
					if string(payload[j+4:j+8]) == "IEND" {
						ln := int(binary.BigEndian.Uint32(payload[j : j+4]))
						end := j + 8 + ln + 4
						if end <= len(payload) {
							img := payload[:end]
							if isValidPNG(img) && decodeValidate(".png", img) {
								h := crc32Key(img)
								if !seen[h] {
									seen[h] = true
									out = append(out, DocImage{Content: append([]byte(nil), img...), Ext: ".png"})
								}
							}
						}
						break
					}
					if j+8 > len(payload) {
						break
					}
					ln := int(binary.BigEndian.Uint32(payload[j : j+4]))
					j += 8 + ln + 4
				}
			}
		case 0xF01F:
			if bmp := convertDIBtoBMP(payload); len(bmp) > 0 && isValidBMP(bmp) {
				h := crc32Key(bmp)
				if !seen[h] {
					seen[h] = true
					out = append(out, DocImage{Content: append([]byte(nil), bmp...), Ext: ".bmp"})
				}
			}
		case 0xF01A, 0xF01B:
			extra := extractFromWMFEMF(payload, seen)
			out = append(out, extra...)
		}
		i = payloadEnd - 1
	}
	return out
}

func extractFromWMFEMF(data []byte, seen map[string]bool) []DocImage {
	var out []DocImage
	for i := 0; i < len(data)-4; i++ {
		if i+22 <= len(data) && data[i] == 0xD7 && data[i+1] == 0xCD && data[i+2] == 0xC6 && data[i+3] == 0x9A {
			start := i
			metaOffset := i + 22
			if metaOffset+18 > len(data) {
				continue
			}
			totalWords := binary.LittleEndian.Uint32(data[metaOffset+6 : metaOffset+10])
			totalBytes := int(totalWords) * 2
			if totalBytes <= 0 || totalBytes > len(data)-start {
				totalBytes = len(data) - start
			}
			wmf := data[start : start+totalBytes]
			imgs := findDIBInWMF(wmf)
			for _, im := range imgs {
				h := crc32Key(im.Content)
				if !seen[h] {
					seen[h] = true
					out = append(out, im)
				}
			}
			i = start + totalBytes - 1
			continue
		}
		if i+18 <= len(data) {
			metaType := binary.LittleEndian.Uint16(data[i : i+2])
			metaHeaderSize := binary.LittleEndian.Uint16(data[i+2 : i+4])
			if (metaType == 0x0001 || metaType == 0x0002) && metaHeaderSize >= 9 {
				totalWords := binary.LittleEndian.Uint32(data[i+6 : i+10])
				totalBytes := int(totalWords) * 2
				if totalBytes > 0 && totalBytes <= len(data)-i {
					wmf := data[i : i+totalBytes]
					imgs := findDIBInWMF(wmf)
					for _, im := range imgs {
						h := crc32Key(im.Content)
						if !seen[h] {
							seen[h] = true
							out = append(out, im)
						}
					}
					i = i + totalBytes - 1
					continue
				}
			}
		}
		if i+88 <= len(data) && data[i] == 0x01 && data[i+1] == 0x00 && data[i+2] == 0x00 && data[i+3] == 0x00 {
			if string(data[i+40:i+44]) == " EMF" {
				totalBytes := int(binary.LittleEndian.Uint32(data[i+48 : i+52]))
				if totalBytes <= 0 || totalBytes > len(data)-i {
					totalBytes = len(data) - i
				}
				emf := data[i : i+totalBytes]
				imgs := findDIBInEMF(emf)
				for _, im := range imgs {
					h := crc32Key(im.Content)
					if !seen[h] {
						seen[h] = true
						out = append(out, im)
					}
				}
				i = i + totalBytes - 1
				continue
			}
		}
	}
	return out
}

func findDIBInWMF(wmf []byte) []DocImage {
	var out []DocImage
	for i := 0; i < len(wmf)-40; i++ {
		if i+4 <= len(wmf) && wmf[i] == 0x28 && wmf[i+1] == 0x00 && wmf[i+2] == 0x00 && wmf[i+3] == 0x00 {
			if bmp := convertDIBtoBMP(wmf[i:]); len(bmp) > 0 {
				out = append(out, DocImage{Content: bmp, Ext: ".bmp"})
			}
		}
	}
	return out
}

func findDIBInEMF(emf []byte) []DocImage { return findDIBInWMF(emf) }

func convertDIBtoBMP(dib []byte) []byte {
	if len(dib) < 40 {
		return nil
	}
	headerSize := binary.LittleEndian.Uint32(dib[0:4])
	if headerSize != 40 && headerSize != 108 && headerSize != 124 {
		return nil
	}
	width := int32(binary.LittleEndian.Uint32(dib[4:8]))
	height := int32(binary.LittleEndian.Uint32(dib[8:12]))
	if width <= 0 || width > 10000 || height == 0 || height < -10000 || height > 10000 {
		return nil
	}
	absHeight := height
	if absHeight < 0 {
		absHeight = -absHeight
	}
	bitCount := binary.LittleEndian.Uint16(dib[14:16])
	if bitCount != 1 && bitCount != 4 && bitCount != 8 && bitCount != 16 && bitCount != 24 && bitCount != 32 {
		return nil
	}
	colorUsed := binary.LittleEndian.Uint32(dib[32:36])
	paletteSize := 0
	if bitCount <= 8 {
		if colorUsed == 0 {
			paletteSize = (1 << bitCount) * 4
		} else {
			paletteSize = int(colorUsed) * 4
		}
	}
	imageSize := int(binary.LittleEndian.Uint32(dib[20:24]))
	if imageSize == 0 {
		rowSize := ((int(width)*int(bitCount) + 31) / 32) * 4
		imageSize = rowSize * int(absHeight)
	}
	totalDIBSize := int(headerSize) + paletteSize + imageSize
	if totalDIBSize > len(dib) {
		totalDIBSize = len(dib)
	}
	bmpFileSize := 14 + totalDIBSize
	bmp := make([]byte, bmpFileSize)
	bmp[0] = 'B'
	bmp[1] = 'M'
	binary.LittleEndian.PutUint32(bmp[2:6], uint32(bmpFileSize))
	binary.LittleEndian.PutUint32(bmp[6:10], 0)
	binary.LittleEndian.PutUint32(bmp[10:14], uint32(14+int(headerSize)+paletteSize))
	copy(bmp[14:], dib[:totalDIBSize])
	return bmp
}

func isValidJPEG(data []byte) bool {
	if len(data) < 100 {
		return false
	}
	if data[0] != 0xFF || data[1] != 0xD8 {
		return false
	}
	if data[len(data)-2] != 0xFF || data[len(data)-1] != 0xD9 {
		return false
	}
	if _, err := jpeg.DecodeConfig(bytes.NewReader(data)); err != nil {
		return false
	}
	return true
}

func isValidPNG(data []byte) bool {
	if len(data) < 8 || data[0] != 0x89 || data[1] != 0x50 || data[2] != 0x4E || data[3] != 0x47 {
		return false
	}
	if _, err := png.DecodeConfig(bytes.NewReader(data)); err != nil {
		return false
	}
	return true
}

func isValidBMP(data []byte) bool {
	if len(data) < 54 {
		return false
	}
	if data[0] != 'B' || data[1] != 'M' {
		return false
	}
	dibSize := binary.LittleEndian.Uint32(data[14:18])
	if dibSize != 40 && dibSize != 108 && dibSize != 124 && dibSize != 12 {
		return false
	}
	return true
}

func decodeValidate(ext string, data []byte) bool {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		_, err := jpeg.DecodeConfig(bytes.NewReader(data))
		return err == nil
	case ".png":
		_, err := png.DecodeConfig(bytes.NewReader(data))
		return err == nil
	case ".gif":
		_, err := gif.DecodeConfig(bytes.NewReader(data))
		return err == nil
	default:
		return len(data) > 0
	}
}

func crc32Key(b []byte) string {
	if len(b) == 0 {
		return "0"
	}
	sample := b
	if len(sample) > 1<<20 {
		sample = sample[:1<<20]
	}
	return fmt.Sprintf("%d-%08x", len(b), crc32.ChecksumIEEE(sample))
}

func extractImagesFromPDFSimple(pdfPath string) ([]DocImage, error) {
	tmpDir, err := os.MkdirTemp("", "bid-engine-pdfimg-")
	if err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := api.ExtractImagesFile(pdfPath, tmpDir, nil, nil); err != nil {
		return nil, fmt.Errorf("提取PDF图片失败: %w", err)
	}

	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return nil, fmt.Errorf("读取临时目录失败: %w", err)
	}

	images := make([]DocImage, 0, len(entries))
	seen := make(map[string]bool)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(tmpDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext == "" {
			ext = ".bin"
		}
		key := crc32Key(data)
		if seen[key] {
			continue
		}
		seen[key] = true
		images = append(images, DocImage{Content: data, Ext: ext})
	}

	return images, nil
}

