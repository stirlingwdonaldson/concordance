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
	Error      string    `json:"error,omitempty"`
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

type PipelineJob struct {
	ID         string    `json:"id"`
	Stage      string    `json:"stage"`
	Status     string    `json:"status"`
	Message    string    `json:"message"`
	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt"`
}

type ConcordanceFilter struct {
	Lemma   string
	POS     string
	Section string
}

type ConcordanceTerm struct {
	ID             int64  `json:"id"`
	Lemma          string `json:"lemma"`
	NormalizedForm string `json:"normalizedForm"`
	TotalFreq      int    `json:"totalFreq"`
	Hapax          bool   `json:"hapax"`
}

type KWICFilter struct {
	Lemma   string
	Page    string
	Section string
	Limit   int
	Offset  int
	SortBy  string
	SortDir string
}

type KWICOccurrence struct {
	ID           int64  `json:"id"`
	TermID       int64  `json:"termId"`
	Lemma        string `json:"lemma"`
	SentenceID   string `json:"sentenceId"`
	LeftContext  string `json:"leftContext"`
	Keyword      string `json:"keyword"`
	RightContext string `json:"rightContext"`
	SectionID    string `json:"sectionId,omitempty"`
	PageRef      string `json:"pageRef,omitempty"`
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
