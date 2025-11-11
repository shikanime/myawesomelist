package app

import (
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"myawesomelist.shikanime.studio/internal/awesome"
)

type Migrator struct {
	db *gorm.DB
}

func NewMigrator(db *gorm.DB) (*Migrator, error) {
	if db == nil {
		return nil, fmt.Errorf("failed to get sql.DB from gorm.DB: %w", fmt.Errorf("nil db"))
	}
	return &Migrator{db: db}, nil
}

func (mg *Migrator) Up() error {
	if mg.db == nil {
		return fmt.Errorf("migrator not initialized")
	}

	if err := mg.db.AutoMigrate(
		&awesome.Repository{},
		&awesome.Collection{},
		&awesome.Category{},
		&awesome.Project{},
		&awesome.ProjectStats{},
	); err != nil {
		return fmt.Errorf("auto-migrate failed: %w", err)
	}

	// Seed default GitHub repositories
	repos := []awesome.Repository{
		{Hostname: "github.com", Owner: "avelino", Repo: "awesome-go"},
		{Hostname: "github.com", Owner: "h4cc", Repo: "awesome-elixir"},
		{Hostname: "github.com", Owner: "sorrycc", Repo: "awesome-javascript"},
		{Hostname: "github.com", Owner: "gostor", Repo: "awesome-go-storage"},
	}
	if err := mg.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "hostname"}, {Name: "owner"}, {Name: "repo"}},
		DoNothing: true,
	}).Create(&repos).Error; err != nil {
		return fmt.Errorf("seed repositories failed: %w", err)
	}

	return nil
}

func (mg *Migrator) Down() error {
	if mg.db == nil {
		return fmt.Errorf("migrator not initialized")
	}

	if err := mg.db.Migrator().DropTable(
		&awesome.Project{},
		&awesome.Category{},
		&awesome.ProjectStats{},
		&awesome.Collection{},
		&awesome.Repository{},
	); err != nil {
		return fmt.Errorf("drop tables failed: %w", err)
	}

	return nil
}
