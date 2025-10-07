package datastore

import (
	"time"
)

// CollectionRecord represents a stored collection in the database
type CollectionRecord struct {
	ID         int64            `json:"id" db:"id"`
	Language   string           `json:"language" db:"language"`
	Owner      string           `json:"owner" db:"owner"`
	Repo       string           `json:"repo" db:"repo"`
	Categories []CategoryRecord `json:"categories"`
	CreatedAt  time.Time        `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time        `json:"updated_at" db:"updated_at"`
}

// CategoryRecord represents a stored category in the database
type CategoryRecord struct {
	ID           int64           `json:"id" db:"id"`
	CollectionID int64           `json:"collection_id" db:"collection_id"`
	Name         string          `json:"name" db:"name"`
	Projects     []ProjectRecord `json:"projects"`
	CreatedAt    time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at" db:"updated_at"`
}

// ProjectRecord represents a stored project in the database
type ProjectRecord struct {
	ID              int64     `json:"id" db:"id"`
	CategoryID      int64     `json:"category_id" db:"category_id"`
	Name            string    `json:"name" db:"name"`
	Description     string    `json:"description" db:"description"`
	URL             string    `json:"url" db:"url"`
	StargazersCount *int      `json:"stargazers_count" db:"stargazers_count"`
	OpenIssueCount  *int      `json:"open_issue_count" db:"open_issue_count"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
}
