package awesome

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
	myawesomelistv1 "myawesomelist.shikanime.studio/pkgs/proto/myawesomelist/v1"
)

type Collection struct {
	ID           uint64     `gorm:"primaryKey"`
	RepositoryID uint64     `gorm:"index;uniqueIndex:uq_collections_repository_id"`
	Repository   Repository `gorm:"constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
	Language     string     `gorm:"size:100;not null;index"`
	CreatedAt    time.Time  `gorm:"autoCreateTime"`
	UpdatedAt    time.Time  `gorm:"autoUpdateTime"`
	Categories   []Category `gorm:"constraint:OnDelete:CASCADE"`
}

func (Collection) TableName() string { return "collections" }

func (m *Collection) ToProto() *myawesomelistv1.Collection {
	pc := &myawesomelistv1.Collection{
		Id: m.ID,
		Repo: &myawesomelistv1.Repository{
			Hostname: m.Repository.Hostname,
			Owner:    m.Repository.Owner,
			Repo:     m.Repository.Repo,
		},
		Language:  m.Language,
		UpdatedAt: timestamppb.New(m.UpdatedAt),
	}
	for _, cat := range m.Categories {
		pc.Categories = append(pc.Categories, cat.ToProto())
	}
	return pc
}

type Category struct {
	ID           uint64    `gorm:"primaryKey"`
	CollectionID uint64    `gorm:"not null;index;uniqueIndex:uq_categories_collection_name"`
	Name         string    `gorm:"size:255;not null;index;uniqueIndex:uq_categories_collection_name"`
	CreatedAt    time.Time `gorm:"autoCreateTime"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime"`
	Projects     []Project `gorm:"constraint:OnDelete:CASCADE"`
}

func (Category) TableName() string { return "categories" }

func (m *Category) ToProto() *myawesomelistv1.Category {
	pc := &myawesomelistv1.Category{
		Id:        m.ID,
		Name:      m.Name,
		UpdatedAt: timestamppb.New(m.UpdatedAt),
	}
	for _, p := range m.Projects {
		pc.Projects = append(pc.Projects, p.ToProto())
	}
	return pc
}

type Project struct {
	ID           uint64     `gorm:"primaryKey"`
	CategoryID   uint64     `gorm:"not null;index;uniqueIndex:uq_projects_category_repository"`
	RepositoryID uint64     `gorm:"not null;uniqueIndex:uq_projects_category_repository"`
	Repository   Repository `gorm:"constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
	Name         string     `gorm:"size:255;not null;index"`
	Description  string     `gorm:"type:text"`
	CreatedAt    time.Time  `gorm:"autoCreateTime"`
	UpdatedAt    time.Time  `gorm:"autoUpdateTime"`
}

func (Project) TableName() string { return "projects" }

func (m *Project) ToProto() *myawesomelistv1.Project {
	return &myawesomelistv1.Project{
		Id:          m.ID,
		Name:        m.Name,
		Description: m.Description,
		Repo: &myawesomelistv1.Repository{
			Hostname: m.Repository.Hostname,
			Owner:    m.Repository.Owner,
			Repo:     m.Repository.Repo,
		},
		UpdatedAt: timestamppb.New(m.UpdatedAt),
	}
}

type ProjectStats struct {
	ID              uint64     `gorm:"primaryKey"`
	RepositoryID    uint64     `gorm:"not null;index;uniqueIndex:uq_project_stats_repository"`
	Repository      Repository `gorm:"constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
	StargazersCount *uint32
	OpenIssueCount  *uint32
	CreatedAt       time.Time `gorm:"autoCreateTime"`
	UpdatedAt       time.Time `gorm:"autoUpdateTime"`
}

func (ProjectStats) TableName() string { return "project_stats" }

func (m *ProjectStats) ToProto() *myawesomelistv1.ProjectStats {
	return &myawesomelistv1.ProjectStats{
		Id:              m.ID,
		StargazersCount: m.StargazersCount,
		OpenIssueCount:  m.OpenIssueCount,
		UpdatedAt:       timestamppb.New(m.UpdatedAt),
	}
}

type ProjectMetadata struct {
	ID           uint64     `gorm:"primaryKey"`
	RepositoryID uint64     `gorm:"not null;index;uniqueIndex:uq_project_metadata_repository"`
	Repository   Repository `gorm:"constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
	Readme       string     `gorm:"type:text"`
	CreatedAt    time.Time  `gorm:"autoCreateTime"`
	UpdatedAt    time.Time  `gorm:"autoUpdateTime"`
}

func (ProjectMetadata) TableName() string { return "project_metadata" }

// Repository GORM model and helpers
type Repository struct {
	ID        uint64    `gorm:"primaryKey"`
	Hostname  string    `gorm:"size:255;not null;index;uniqueIndex:uq_repositories_hostname_owner_repo"`
	Owner     string    `gorm:"size:255;not null;index;uniqueIndex:uq_repositories_hostname_owner_repo"`
	Repo      string    `gorm:"size:255;not null;index;uniqueIndex:uq_repositories_hostname_owner_repo"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

func (Repository) TableName() string { return "repositories" }

func (m *Repository) ToProto() *myawesomelistv1.Repository {
	return &myawesomelistv1.Repository{
		Hostname: m.Hostname,
		Owner:    m.Owner,
		Repo:     m.Repo,
	}
}
