package entity

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

type MaterialTypeOption struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type MaterialCompanyOption struct {
	CompanyID int32  `json:"company_id"`
	Name      string `json:"name"`
}

type MaterialUserOption struct {
	UserID int64  `json:"user_id"`
	Name   string `json:"name"`
}

type MaterialListReq struct {
	PageNum   int    `form:"pageNum" json:"pageNum"`
	PageSize  int    `form:"pageSize" json:"pageSize"`
	Keyword   string `form:"keyword" json:"keyword"`
	Type      string `form:"type" json:"type"`
	CompanyID int32  `form:"companyId" json:"companyId"`
	CreatorID int64  `form:"creatorId" json:"creatorId"`
}

func (r *MaterialListReq) Valid(c *gin.Context) error {
	if err := c.ShouldBindQuery(r); err != nil {
		return err
	}
	if r.PageNum <= 0 {
		r.PageNum = 1
	}
	if r.PageSize <= 0 {
		r.PageSize = 10
	}
	if r.PageSize > 100 {
		r.PageSize = 100
	}
	r.Keyword = strings.TrimSpace(r.Keyword)
	r.Type = strings.TrimSpace(r.Type)
	return nil
}

type MaterialListItem struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	TypeName    string `json:"type_name"`
	Description string `json:"description"`
	FileCount   int64  `json:"file_count"`
	CompanyID   int32  `json:"company_id"`
	CompanyName string `json:"company_name"`
	UserID      int64  `json:"user_id"`
	UserName    string `json:"user_name"`
	CreatedTime string `json:"created_time"`
}

type MaterialListResp struct {
	List  []*MaterialListItem `json:"list"`
	Total int64               `json:"total"`
}

type MaterialAddReq struct {
	Name        string `form:"name" json:"name"`
	Type        string `form:"type" json:"type"`
	Description string `form:"description" json:"description"`
	AutoAnalysis bool  `form:"autoAnalysis" json:"autoAnalysis"`
}

func (r *MaterialAddReq) Valid(c *gin.Context) error {
	if err := c.ShouldBind(r); err != nil {
		return err
	}
	r.Name = strings.TrimSpace(r.Name)
	r.Type = strings.TrimSpace(r.Type)
	r.Description = strings.TrimSpace(r.Description)
	if r.Name == "" {
		return errors.New("name 不能为空")
	}
	if !regexp.MustCompile(`^[\p{Han}a-zA-Z0-9_-]+$`).MatchString(r.Name) {
		return errors.New("name 仅支持汉字、字母、数字、中划线、下划线")
	}
	if len([]rune(r.Description)) > 1000 {
		return fmt.Errorf("description 超过最大长度 1000")
	}
	return nil
}

type MaterialUpdateReq struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

func (r *MaterialUpdateReq) Valid(c *gin.Context) error {
	if err := c.ShouldBindJSON(r); err != nil {
		return err
	}
	r.Name = strings.TrimSpace(r.Name)
	r.Type = strings.TrimSpace(r.Type)
	r.Description = strings.TrimSpace(r.Description)
	if r.Name == "" {
		return errors.New("name 不能为空")
	}
	if r.Type == "" {
		return errors.New("type 不能为空")
	}
	if len([]rune(r.Description)) > 1000 {
		return fmt.Errorf("description 超过最大长度 1000")
	}
	return nil
}

var nameRegex = regexp.MustCompile(`^[\p{Han}a-zA-Z0-9_-]+$`)

// ValidateName 校验素材名称（仅汉字、字母、数字、中划线、下划线）
func ValidateName(name string) error {
	if name == "" {
		return errors.New("name 不能为空")
	}
	if !nameRegex.MatchString(name) {
		return errors.New("name 仅支持汉字、字母、数字、中划线、下划线")
	}
	return nil
}

type MaterialIDUri struct {
	ID int64 `uri:"id"`
}

func (r *MaterialIDUri) Valid(c *gin.Context) error {
	if err := c.ShouldBindUri(r); err != nil {
		return err
	}
	if r.ID <= 0 {
		return errors.New("id 参数错误")
	}
	return nil
}

type MaterialFileIDUri struct {
	ID int64 `uri:"id"`
}

func (r *MaterialFileIDUri) Valid(c *gin.Context) error {
	if err := c.ShouldBindUri(r); err != nil {
		return err
	}
	if r.ID <= 0 {
		return errors.New("id 参数错误")
	}
	return nil
}

type MaterialFilePreviewUri struct {
	ID int64 `uri:"id"`
}

func (r *MaterialFilePreviewUri) Valid(c *gin.Context) error {
	if err := c.ShouldBindUri(r); err != nil {
		return err
	}
	if r.ID <= 0 {
		return errors.New("id 参数错误")
	}
	return nil
}

type MaterialFileAddUri struct {
	ID int64 `uri:"id"`
}

func (r *MaterialFileAddUri) Valid(c *gin.Context) error {
	if err := c.ShouldBindUri(r); err != nil {
		return err
	}
	if r.ID <= 0 {
		return errors.New("id 参数错误")
	}
	return nil
}

type MaterialImageIDUri struct {
	ID int64 `uri:"id"`
}

func (r *MaterialImageIDUri) Valid(c *gin.Context) error {
	if err := c.ShouldBindUri(r); err != nil {
		return err
	}
	if r.ID <= 0 {
		return errors.New("id 参数错误")
	}
	return nil
}

type MaterialFileAddReq struct {
	AutoAnalysis bool `form:"autoAnalysis" json:"autoAnalysis"`
}

func (r *MaterialFileAddReq) Valid(c *gin.Context) error {
	if err := c.ShouldBind(r); err != nil {
		return err
	}
	return nil
}

type MaterialSortOrderReq struct {
	MaterialID int64   `json:"material_id"`
	FileIDs    []int64 `json:"file_ids"`
}

func (r *MaterialSortOrderReq) Valid(c *gin.Context) error {
	if err := c.ShouldBindJSON(r); err != nil {
		return err
	}
	if r.MaterialID <= 0 {
		return errors.New("material_id 参数错误")
	}
	if len(r.FileIDs) == 0 {
		return errors.New("file_ids 不能为空")
	}
	seen := make(map[int64]struct{}, len(r.FileIDs))
	out := make([]int64, 0, len(r.FileIDs))
	for _, id := range r.FileIDs {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	if len(out) == 0 {
		return errors.New("file_ids 参数错误")
	}
	r.FileIDs = out
	return nil
}

type MaterialFileItem struct {
	ID          int64  `json:"id"`
	MaterialID  int64  `json:"material_id"`
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	ObjectKey   string `json:"object_key"`
	SortOrder   int32  `json:"sort_order"`
	CreatedTime int64  `json:"created_time"`
	MaterialFileID  int64  `json:"material_file_id,omitempty"`
	OriginObjectKey string `json:"origin_object_key,omitempty"`
	EditFlow        string `json:"edit_flow"`
}

type MaterialImageItem struct {
	ID             int64  `json:"id"`
	MaterialID     int64  `json:"material_id"`
	MaterialFileID int64  `json:"material_file_id"`
	Name           string `json:"name"`
	Size           int64  `json:"size"`
	OriginObjectKey string `json:"origin_object_key"`
	ObjectKey      string `json:"object_key"`
	SortOrder      int32  `json:"sort_order"`
	EditFlow       string `json:"edit_flow"`
	CreatedTime    int64  `json:"created_time"`
}

type MaterialDetailResp struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	TypeName    string `json:"type_name"`
	Description string `json:"description"`
	UserID      int64  `json:"user_id"`
	UserName    string `json:"user_name"`
	CompanyID   int32  `json:"company_id"`
	CompanyName string `json:"company_name"`
	CreatedTime string `json:"created_time"`

	ImageFiles []*MaterialFileItem `json:"image_files"`
	DocFiles   []*MaterialFileItem `json:"doc_files"`
}

type MaterialGalleryGroup struct {
	MaterialID   int64              `json:"material_id"`
	MaterialName string             `json:"material_name"`
	Images       []*MaterialImageItem `json:"images"`
}

type MaterialGalleryResp struct {
	Groups []*MaterialGalleryGroup `json:"groups"`
}

type MaterialGalleryDeleteReq struct {
	ImageIDs []int64 `json:"image_ids"`
}

func (r *MaterialGalleryDeleteReq) Valid(c *gin.Context) error {
	if err := c.ShouldBindJSON(r); err != nil {
		return err
	}
	out := make([]int64, 0, len(r.ImageIDs))
	seen := make(map[int64]struct{}, len(r.ImageIDs))
	for _, id := range r.ImageIDs {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	if len(out) == 0 {
		return errors.New("image_ids 不能为空")
	}
	r.ImageIDs = out
	return nil
}

type MaterialGalleryMoveReq struct {
	ImageID      int64 `json:"image_id"`
	ToMaterialID int64 `json:"to_material_id"`
}

func (r *MaterialGalleryMoveReq) Valid(c *gin.Context) error {
	if err := c.ShouldBindJSON(r); err != nil {
		return err
	}
	if r.ImageID <= 0 {
		return errors.New("image_id 参数错误")
	}
	if r.ToMaterialID < 0 {
		return errors.New("to_material_id 参数错误")
	}
	return nil
}

type NewEditorImage struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type NewEditorTextItem struct {
	ID       string  `json:"id"`
	Text     string  `json:"text"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Color    string  `json:"color"`
	FontSize float64 `json:"fontSize"`
}

type NewEditorShapeItem struct {
	ID         string  `json:"id"`
	Type       string  `json:"type"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	W          float64 `json:"w"`
	H          float64 `json:"h"`
	Color      string  `json:"color"`
	LineWidth  float64 `json:"lineWidth"`
	LineDash   []int   `json:"lineDash"`
	Fill       string  `json:"fill"`
	MosaicSize int     `json:"mosaicSize"`
}

type NewEditorImageFlow struct {
	Image  NewEditorImage       `json:"image"`
	Texts  []NewEditorTextItem  `json:"texts"`
	Shapes []NewEditorShapeItem `json:"shapes"`
}

func ParseNewEditorImageFlow(s string) (*NewEditorImageFlow, error) {
	if strings.TrimSpace(s) == "" {
		return nil, fmt.Errorf("操作流内容为空")
	}
	var flow NewEditorImageFlow
	if err := json.Unmarshal([]byte(s), &flow); err != nil {
		return nil, err
	}
	return &flow, nil
}
