package database

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"myawesomelist.shikanime.studio/internal/config"
	dbpgx "myawesomelist.shikanime.studio/internal/database/pgx"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrator applies SQL migrations to the database.
type Migrator struct {
	pg *pgxpool.Pool
}

// NewMigrator constructs a Migrator using an existing pgx pool.
func NewMigrator(pg *pgxpool.Pool) (*Migrator, error) {
	if pg == nil {
		return nil, fmt.Errorf("nil pgx pool")
	}
	return &Migrator{pg: pg}, nil
}

// NewMigratorForConf constructs a Migrator from configuration by creating a pgx pool internally.
func NewMigratorForConf(cfg *config.Config) (*Migrator, error) {
	pg, err := dbpgx.NewClientForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return NewMigrator(pg)
}

// Up applies all pending migrations.
func (mg *Migrator) Up() error {
	if mg.pg == nil {
		return fmt.Errorf("migrator not initialized")
	}
	driver, err := pgx.WithInstance(sql.OpenDB(stdlib.GetPoolConnector(mg.pg)), &pgx.Config{})
	if err != nil {
		return err
	}
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return err
	}
	m, err := migrate.NewWithInstance("iofs", src, "pgx", driver)
	if err != nil {
		return err
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}

// Down reverts all applied migrations.
func (mg *Migrator) Down() error {
	if mg.pg == nil {
		return fmt.Errorf("migrator not initialized")
	}
	driver, err := pgx.WithInstance(sql.OpenDB(stdlib.GetPoolConnector(mg.pg)), &pgx.Config{})
	if err != nil {
		return err
	}
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return err
	}
	m, err := migrate.NewWithInstance("iofs", src, "pgx", driver)
	if err != nil {
		return err
	}
	if err := m.Down(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}
