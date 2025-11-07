package app

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations
var migrations embed.FS

type Migrator struct {
	m *migrate.Migrate
}

func NewMigrator(db *sql.DB) (*Migrator, error) {
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to create postgres driver: %w", err)
	}
	src, err := iofs.New(migrations, "migrations")
	if err != nil {
		return nil, fmt.Errorf("failed to init iofs source: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "postgres", driver)
	if err != nil {
		return nil, fmt.Errorf("failed to init migrator: %w", err)
	}
	return &Migrator{m: m}, nil
}

func (mg *Migrator) Up() error {
	if mg.m == nil {
		return fmt.Errorf("migrator not initialized")
	}
	if err := mg.m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migration up failed: %w", err)
	}
	return nil
}

func (mg *Migrator) Down() error {
	if mg.m == nil {
		return fmt.Errorf("migrator not initialized")
	}
	if err := mg.m.Down(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migration down failed: %w", err)
	}
	return nil
}

func (mg *Migrator) Drop() error {
	if mg.m == nil {
		return fmt.Errorf("migrator not initialized")
	}
	return mg.m.Drop()
}

func (mg *Migrator) Close() error {
	if mg.m == nil {
		return nil
	}
	srcErr, dbErr := mg.m.Close()
	if srcErr != nil && dbErr != nil {
		return fmt.Errorf("close errors: source=%v, db=%v", srcErr, dbErr)
	}
	if srcErr != nil {
		return srcErr
	}
	return dbErr
}
