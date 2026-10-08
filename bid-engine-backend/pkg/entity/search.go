package entity

// NewSearchParam 搜索参数
type NewSearchParam struct {
	Name     string
	Status   int32
	PageSize int
	PageNum  int
}

type SearchUserParam struct {
	Query     string
	Statuses  []int32
	StartTime int64
	EndTime   int64
	PageSize  int
	PageNum   int
}

type SearchParam struct {
	Query    string
	Statuses []int32
	PageSize int
	PageNum  int
}
