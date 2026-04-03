package documents

import "time"

type Document struct {
	ID         string    `json:"id"`
	ProjectID  string    `json:"projectId"`
	FileName   string    `json:"fileName"`
	LocalPath  string    `json:"localPath"`
	SourceHash string    `json:"-"`
	Format     string    `json:"format"`
	Status     string    `json:"status"`
	Progress   float64   `json:"progress"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type Passage struct {
	ID            string `json:"id"`
	PassageIndex  int    `json:"passageIndex"`
	Text          string `json:"text"`
	StartChar     int64  `json:"startChar"`
	EndChar       int64  `json:"endChar"`
	SentenceCount int    `json:"sentenceCount"`
}

type Sentence struct {
	ID            string `json:"id"`
	PassageID     string `json:"passageId"`
	SentenceIndex int    `json:"sentenceIndex"`
	Text          string `json:"text"`
	StartChar     int64  `json:"startChar"`
	EndChar       int64  `json:"endChar"`
}

type JobStage struct {
	Name      string    `json:"name"`
	StartedAt time.Time `json:"startedAt"`
	EndedAt   time.Time `json:"endedAt,omitempty"`
}

type CreateInput struct {
	ProjectID  string
	FileName   string
	LocalPath  string
	SourceHash string
	Format     string
}
