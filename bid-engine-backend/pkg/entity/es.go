package entity

import "time"

// ESResp es返回结果
type ESResp struct {
	Hits ESHits `json:"hits"`
}

// ESHits ES检索结果
type ESHits struct {
	Total HitsTotal `json:"total"`
	Hits  []NewsHit `json:"hits"`
}

// HitsTotal 命中条数
type HitsTotal struct {
	Value int32 `json:"value"`
}

// NewsHit ES检索单个文档结果
type NewsHit struct {
	Index  string   `json:"_index"`
	Type   string   `json:"_type"`
	ID     string   `json:"_id"`
	Score  float64  `json:"_score"`
	Source ESSource `json:"_source"`
}

// ESSource ES原始数据
type ESSource struct {
	DocID           string    `json:"doc_id"`
	Title           string    `json:"title"`
	Content         string    `json:"content"`
	ParagraphID     int       `json:"paragraph_id"`
	PageNum         int64     `json:"page_num"`
	ParagraphVector []float64 `json:"paragraph_vector"`
	UploadTime      time.Time `json:"upload_time"`
}

// ESHistoryResp es返回结果
type ESHistoryResp struct {
	Hits ESHistoryHits `json:"hits"`
}

// ESHistoryHits ES检索结果
type ESHistoryHits struct {
	Total HitsTotal     `json:"total"`
	Hits  []*HistoryHit `json:"hits"`
}

// HistoryHit ES检索单个message结果
type HistoryHit struct {
	Index     string          `json:"_index"`
	Type      string          `json:"_type"`
	ID        string          `json:"_id"`
	Score     float64         `json:"_score"`
	Source    ESHistorySource `json:"_source"`
	Highlight Highlight       `json:"highlight"`
}

// ESHistorySource ES原始数据
type ESHistorySource struct {
	MessageID      string    `json:"message_id"`
	ConversationID string    `json:"conversation_id"`
	UserID         int64     `json:"user_id"`
	Query          string    `json:"query"`
	Answer         string    `json:"answer"`
	Title          string    `json:"title"`
	CreateTime     time.Time `json:"create_time"`
	UpdateTime     time.Time `json:"update_time"`
}

type Highlight struct {
	Query  []string `json:"query"`
	Answer []string `json:"answer"`
	Title  []string `json:"title"`
}
