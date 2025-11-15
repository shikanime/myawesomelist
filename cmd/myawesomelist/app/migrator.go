package app

import (
	"fmt"
	"log/slog"

	"gorm.io/gorm"
	"myawesomelist.shikanime.studio/internal/awesome"
)

type Migrator struct {
	db *gorm.DB
}

func NewMigrator(db *gorm.DB) (*Migrator, error) {
	if db == nil {
		return nil, fmt.Errorf("failed to get sql.DB from gorm.DB: %w", fmt.Errorf("nil db"))
	}
	slog.Debug("migrator initialized")
	return &Migrator{db: db}, nil
}

func (mg *Migrator) Up() error {
	if mg.db == nil {
		return fmt.Errorf("migrator not initialized")
	}

	slog.Info("migrate up: start")
	if err := mg.db.Exec("CREATE EXTENSION IF NOT EXISTS vector;").Error; err != nil {
		slog.Error("create vector extension failed", "error", err)
		return fmt.Errorf("create vector extension failed: %w", err)
	}
	slog.Debug("vector extension ensured")

	if err := mg.db.AutoMigrate(
		&awesome.Repository{},
		&awesome.Collection{},
		&awesome.Category{},
		&awesome.Project{},
		&awesome.ProjectStats{},
		&awesome.ProjectEmbeddings{},
		&awesome.ProjectMetadata{},
	); err != nil {
		slog.Error("auto-migrate failed", "error", err)
		return fmt.Errorf("auto-migrate failed: %w", err)
	}

	slog.Info("migrate up: done")
	return nil
}

func (mg *Migrator) Down() error {
	if mg.db == nil {
		return fmt.Errorf("migrator not initialized")
	}

	slog.Info("migrate down: start")
	if err := mg.db.Migrator().DropTable(
		&awesome.Repository{},
		&awesome.Collection{},
		&awesome.Category{},
		&awesome.Project{},
		&awesome.ProjectStats{},
		&awesome.ProjectEmbeddings{},
		&awesome.ProjectMetadata{},
	); err != nil {
		slog.Error("drop tables failed", "error", err)
		return fmt.Errorf("drop tables failed: %w", err)
	}

	if err := mg.db.Exec("DROP EXTENSION IF EXISTS vector;").Error; err != nil {
		slog.Error("drop vector extension failed", "error", err)
		return fmt.Errorf("drop vector extension failed: %w", err)
	}

	slog.Info("migrate down: done")
	return nil
}
