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
