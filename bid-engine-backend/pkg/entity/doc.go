package entity

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"bid-engine/lib/common/config"

	"github.com/google/uuid"
)

const (
	// CosFileBase 对象存储base路径
	CosFileBase = "bid-engine"
)

// DocParseStatus 文档类型
type DocParseStatus = int32

const (
	// DocStatusUploaded 已上传
	DocStatusUploaded DocParseStatus = 0
	// DocStatusParseSuccess 解析完成
	DocStatusParseSuccess DocParseStatus = 1
	// DocStatusInProgress 解析中
	DocStatusInProgress DocParseStatus = 2
	// DocStatusParseError 解析失败
	DocStatusParseError DocParseStatus = 3
)

// DocType 文档类型
type DocType = int32

const (
	// DocTypePDF PDF
	DocTypePDF DocType = 1
	// DocTypeDocx docx
	DocTypeDocx DocType = 2
	// DocTypeDoc doc
	DocTypeDoc DocType = 3
	// DocTypePPTx pptx
	DocTypePPTx DocType = 4
	// DocTypePPT ppt
	DocTypePPT DocType = 5
	// DocTypeXLSX Excel
	DocTypeXLSX DocType = 6
	// DocTypeXLS Excel
	DocTypeXLS DocType = 7
	// DocTypeCSV csv
	DocTypeCSV DocType = 8
	// DocTypeTXT txt
	DocTypeTXT DocType = 9
	// DocTypeJPG jpg
	DocTypeJPG DocType = 10
	// DocTypePNG png
	DocTypePNG DocType = 11
	// DocTypeMD markdown
	DocTypeMD DocType = 12
	// DocTypeJPEG jpeg
	DocTypeJPEG DocType = 13

	// DocTypeURL url
	DocTypeURL DocType = 100

	// DocTypeMp4 mp4
	DocTypeMp4 DocType = 50
	// DocTypeWmv wmv
	DocTypeWmv DocType = 51
	// DocTypeMkv mkv
	DocTypeMkv DocType = 52
	// DocTypeAvi avi
	DocTypeAvi DocType = 53
	// DocTypeFlv flv
	DocTypeFlv DocType = 54
	// DocTypeM4s m4s
	DocTypeM4s DocType = 55
	// DocTypeWav wav
	DocTypeWav DocType = 80
	// DocTypeMp3 mp3
	DocTypeMp3 DocType = 81
	// DocTypeM4a m4a
	DocTypeM4a DocType = 82
	DocTypeEml DocType = 83

	DocTypeVideo DocType = 98
	DocTypeAudio DocType = 99

	//// DocTypeURLPDF url
	//DocTypeURLPDF DocType = 101
)

const (
	// DocExtPDF pdf后缀
	DocExtPDF = ".pdf"
	// DocExtDocx docx后缀
	DocExtDocx = ".docx"
	// DocExtDoc doc后缀
	DocExtDoc = ".doc"
	// DocExtPPTx pptx后缀
	DocExtPPTx = ".pptx"
	// DocExtPPT ppt后缀
	DocExtPPT = ".ppt"
	// DocExtXLSX Excel
	DocExtXLSX = ".xlsx"
	// DocExtXLS Excel
	DocExtXLS = ".xls"
	// DocExtCSV csv
	DocExtCSV = ".csv"
	// DocExtTXT txt
	DocExtTXT = ".txt"
	// DocExtJPG jpg
	DocExtJPG = ".jpg"
	// DocExtJPEG jpeg
	DocExtJPEG = ".jpeg"
	// DocExtPNG png
	DocExtPNG = ".png"
	// DocExtMD md
	DocExtMD = ".md"
	// DocExtURL url
	DocExtURL = "URL"
	// DocExtMp4 mp4
	DocExtMp4 = ".mp4"
	// DocExtWmv wmv
	DocExtWmv = ".wmv"
	// DocExtMkv mkv
	DocExtMkv = ".mkv"
	// DocExtAvi avi
	DocExtAvi = ".avi"
	// DocExtFlv flv
	DocExtFlv = ".flv"
	// DocExtM4s m4s
	DocExtM4s = ".m4s"
	// DocExtWav wav
	DocExtWav = ".wav"
	// DocExtMp3 mp3
	DocExtMp3 = ".mp3"
	// DocExtM4a m4a
	DocExtM4a = ".m4a"
	DocExtEml = ".eml"
	//DocExtURLPdf url
	//DocExtURLPdf = "URLPdf"

)

var (
	// DocExt2Type 后缀->文档类型
	DocExt2Type = map[string]DocType{
		DocExtPDF:  DocTypePDF,
		DocExtDocx: DocTypeDocx,
		DocExtDoc:  DocTypeDoc,
		DocExtPPTx: DocTypePPTx,
		DocExtPPT:  DocTypePPT,
		DocExtXLSX: DocTypeXLSX,
		DocExtXLS:  DocTypeXLS,
		DocExtCSV:  DocTypeCSV,
		DocExtTXT:  DocTypeTXT,
		DocExtJPG:  DocTypeJPG,
		DocExtJPEG: DocTypeJPEG,
		DocExtPNG:  DocTypePNG,
		DocExtMD:   DocTypeMD,
		DocExtURL:  DocTypeURL,
		DocExtMp4:  DocTypeMp4,
		DocExtWmv:  DocTypeWmv,
		DocExtMkv:  DocTypeMkv,
		DocExtAvi:  DocTypeAvi,
		DocExtFlv:  DocTypeFlv,
		DocExtM4s:  DocTypeM4s,
		DocExtWav:  DocTypeWav,
		DocExtMp3:  DocTypeMp3,
		DocExtM4a:  DocTypeM4a,
		DocExtEml:  DocTypeEml,
		//DocExtURLPdf: DocTypeURLPDF,
	}
	// DocType2Ext 文档类型->后缀
	DocType2Ext = map[DocType]string{
		DocTypePDF:  DocExtPDF,
		DocTypeDocx: DocExtDocx,
		DocTypeDoc:  DocExtDoc,
		DocTypePPTx: DocExtPPTx,
		DocTypePPT:  DocExtPPT,
		DocTypeXLSX: DocExtXLSX,
		DocTypeXLS:  DocExtXLS,
		DocTypeCSV:  DocExtCSV,
		DocTypeTXT:  DocExtTXT,
		DocTypeJPG:  DocExtJPG,
		DocTypeJPEG: DocExtJPEG,
		DocTypePNG:  DocExtPNG,
		DocTypeMD:   DocExtMD,
		DocTypeURL:  DocExtURL,
		DocTypeMp4:  DocExtMp4,
		DocTypeWmv:  DocExtWmv,
		DocTypeMkv:  DocExtMkv,
		DocTypeAvi:  DocExtAvi,
		DocTypeFlv:  DocExtFlv,
		DocTypeM4s:  DocExtM4s,
		DocTypeWav:  DocExtWav,
		DocTypeMp3:  DocExtMp3,
		DocTypeM4a:  DocExtM4a,
		DocTypeEml:  DocExtEml,

		//DocTypeURLPDF: DocExtURLPdf,
	}
)

// IsMeetingFile 音视频文档
func IsMeetingFile(docType DocType) bool {
	return docType == DocTypeMp4 || docType == DocTypeWmv || docType == DocTypeMkv || docType == DocTypeAvi ||
		docType == DocTypeFlv || docType == DocTypeWav || docType == DocTypeMp3 || docType == DocTypeM4a
}

// IsExcelFile 表格类文档
func IsExcelFile(docType DocType) bool {
	return docType == DocTypeCSV || docType == DocTypeXLSX || docType == DocTypeXLS
}

// GetFilePathByFileIDBasePath 通过文件id构造文件路径
func GetFilePathByFileIDBasePath(FileID string, ext string) string {
	uuidBytes := []byte(FileID)
	var u uuid.UUID
	decodeHex(u[:], uuidBytes)
	path := fmt.Sprintf("%s/%s/%s/%s",
		GetOSSBasePath(),
		randomStr(binary.BigEndian.Uint32(u[0:4])),
		randomStr(binary.BigEndian.Uint32(u[4:8])),
		FileID)
	if ext != "" {
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
			ext = strings.ToLower(ext)
		}
		path += ext
	}
	return path
}

// GetOSSBasePath 获取对象存储根目录
func GetOSSBasePath() string {
	if config.GetConfig().GetProperty("oss_base") != "" {
		return config.GetConfig().GetProperty("oss_base")
	}
	return CosFileBase
}

func decodeHex(uuid []byte, src []byte) {
	_, _ = hex.Decode(uuid[0:4], src)
	_, _ = hex.Decode(uuid[4:6], src[9:13])
	_, _ = hex.Decode(uuid[6:8], src[14:18])
	_, _ = hex.Decode(uuid[8:10], src[19:23])
	_, _ = hex.Decode(uuid[10:], src[24:])
}

func randomStr(r uint32) string {
	r = r*1664525 + 1013904223 // constants from Numerical Recipes
	return strconv.Itoa(int(1000 + r%1000))[1:]
}

// DocSearchInfo 文档搜索结果
type DocSearchInfo struct {
	DocID           string           `json:"doc_id"`
	DirectoryID     int32            `json:"directory_id"`
	LibraryID       int32            `json:"library_id"`
	Title           string           `json:"title,omitempty"`
	ParagraphVector []float64        `json:"paragraph_vector,omitempty"`
	UserID          int64            `json:"user_id"`
	Status          int32            `json:"status"`
	CompanyID       int32            `json:"company_id"`
	PubType         int32            `json:"pub_type"`
	Paragraph       *SearchParagraph `json:"paragraph,omitempty"`
	UploadTime      time.Time        `json:"upload_time"`
	Question        string           `json:"question,omitempty"`
	Answer          string           `json:"answer,omitempty"`
	DocIDs          []string         `json:"doc_ids,omitempty"`
	LibraryIDs      []int32          `json:"library_ids,omitempty"`
	TitleVector     []float64        `json:"title_vector,omitempty"`
	QaLibraryID     int32            `json:"qa_library_id,omitempty"`
	ExpireTime      time.Time        `json:"expire_time"`
	Version         int64            `json:"version"`
}

// SearchParagraph 文档段落信息
type SearchParagraph struct {
	Pid           int32  `json:"pid"`
	Text          string `json:"text"`
	StartTime     int64  `json:"start_time"`
	EndTime       int64  `json:"end_time"`
	PageNum       int64  `json:"page_num"`
	ParagraphType int    `json:"paragraph_type"` // 0 文本，1 图片
}

const NodesDisposition = `attachment; filename="`
