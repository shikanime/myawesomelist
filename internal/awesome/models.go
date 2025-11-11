package awesome

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
	myawesomelistv1 "myawesomelist.shikanime.studio/pkgs/proto/myawesomelist/v1"
)

type Collection struct {
	ID         uint64     `gorm:"primaryKey"`
	Hostname   string     `gorm:"size:255;not null;index;uniqueIndex:uq_collections_hostname_owner_repo"`
	Owner      string     `gorm:"size:255;not null;index;uniqueIndex:uq_collections_hostname_owner_repo"`
	Repo       string     `gorm:"size:255;not null;index;uniqueIndex:uq_collections_hostname_owner_repo"`
	Language   string     `gorm:"size:100;not null;index"`
	CreatedAt  time.Time  `gorm:"autoCreateTime"`
	UpdatedAt  time.Time  `gorm:"autoUpdateTime"`
	Categories []Category `gorm:"constraint:OnDelete:CASCADE"`
}

func (Collection) TableName() string { return "collections" }

func (m *Collection) ToProto() *myawesomelistv1.Collection {
	pc := &myawesomelistv1.Collection{
		Id: m.ID,
		Repo: &myawesomelistv1.Repository{
			Hostname: m.Hostname,
			Owner:    m.Owner,
			Repo:     m.Repo,
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
	ID          uint64    `gorm:"primaryKey"`
	CategoryID  uint64    `gorm:"not null;index;uniqueIndex:uq_projects_category_repo"`
	Hostname    string    `gorm:"size:255;not null;uniqueIndex:uq_projects_category_repo"`
	Owner       string    `gorm:"size:255;not null;uniqueIndex:uq_projects_category_repo"`
	Repo        string    `gorm:"size:255;not null;uniqueIndex:uq_projects_category_repo"`
	Name        string    `gorm:"size:255;not null;index"`
	Description string    `gorm:"type:text"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

func (Project) TableName() string { return "projects" }

func (m *Project) ToProto() *myawesomelistv1.Project {
	return &myawesomelistv1.Project{
		Id:          m.ID,
		Name:        m.Name,
		Description: m.Description,
		Repo: &myawesomelistv1.Repository{
			Hostname: m.Hostname,
			Owner:    m.Owner,
			Repo:     m.Repo,
		},
		UpdatedAt: timestamppb.New(m.UpdatedAt),
	}
}

type ProjectStats struct {
	ID              uint64 `gorm:"primaryKey"`
	Hostname        string `gorm:"size:255;not null;index;uniqueIndex:uq_project_stats_repo"`
	Owner           string `gorm:"size:255;not null;index;uniqueIndex:uq_project_stats_repo"`
	Repo            string `gorm:"size:255;not null;index;uniqueIndex:uq_project_stats_repo"`
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
