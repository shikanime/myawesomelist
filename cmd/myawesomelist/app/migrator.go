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
		&awesome.ProjectMetadata{},
	); err != nil {
		return fmt.Errorf("auto-migrate failed: %w", err)
	}

	var cols []awesome.Collection
	if err := mg.db.Find(&cols).Error; err != nil {
		return fmt.Errorf("backfill load collections failed: %w", err)
	}
	for _, c := range cols {
		var r awesome.Repository
		if err := mg.db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "hostname"}, {Name: "owner"}, {Name: "repo"}},
			DoNothing: true,
		}).Create(&awesome.Repository{Hostname: c.Repository.Hostname, Owner: c.Repository.Owner, Repo: c.Repository.Repo}).Error; err != nil {
			return fmt.Errorf("backfill upsert repo for collection failed: %w", err)
		}
		if err := mg.db.Where("hostname = ? AND owner = ? AND repo = ?", c.Repository.Hostname, c.Repository.Owner, c.Repository.Repo).First(&r).Error; err != nil {
			return fmt.Errorf("backfill load repo for collection failed: %w", err)
		}
		_ = mg.db.Model(&awesome.Collection{}).
			Where("id = ?", c.ID).
			Update("repository_id", r.ID).
			Error
	}

	// Projects -> Repository
	var ps []awesome.Project
	if err := mg.db.Find(&ps).Error; err != nil {
		return fmt.Errorf("backfill load projects failed: %w", err)
	}
	for _, p := range ps {
		var r awesome.Repository
		if err := mg.db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "hostname"}, {Name: "owner"}, {Name: "repo"}},
			DoNothing: true,
		}).Create(&awesome.Repository{Hostname: p.Repository.Hostname, Owner: p.Repository.Owner, Repo: p.Repository.Repo}).Error; err != nil {
			return fmt.Errorf("backfill upsert repo for project failed: %w", err)
		}
		if err := mg.db.Where("hostname = ? AND owner = ? AND repo = ?", p.Repository.Hostname, p.Repository.Owner, p.Repository.Repo).First(&r).Error; err != nil {
			return fmt.Errorf("backfill load repo for project failed: %w", err)
		}
		_ = mg.db.Model(&awesome.Project{}).
			Where("id = ?", p.ID).
			Update("repository_id", r.ID).
			Error
	}

	// ProjectStats -> Repository
	var stats []awesome.ProjectStats
	if err := mg.db.Find(&stats).Error; err != nil {
		return fmt.Errorf("backfill load project_stats failed: %w", err)
	}
	for _, s := range stats {
		var r awesome.Repository
		if err := mg.db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "hostname"}, {Name: "owner"}, {Name: "repo"}},
			DoNothing: true,
		}).Create(&awesome.Repository{Hostname: s.Repository.Hostname, Owner: s.Repository.Owner, Repo: s.Repository.Repo}).Error; err != nil {
			return fmt.Errorf("backfill upsert repo for project_stats failed: %w", err)
		}
		if err := mg.db.Where("hostname = ? AND owner = ? AND repo = ?", s.Repository.Hostname, s.Repository.Owner, s.Repository.Repo).First(&r).Error; err != nil {
			return fmt.Errorf("backfill load repo for project_stats failed: %w", err)
		}
		_ = mg.db.Model(&awesome.ProjectStats{}).
			Where("id = ?", s.ID).
			Update("repository_id", r.ID).
			Error
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
		&awesome.ProjectMetadata{},
		&awesome.Collection{},
		&awesome.Repository{},
	); err != nil {
		return fmt.Errorf("drop tables failed: %w", err)
	}

	return nil
}
