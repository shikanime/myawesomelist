package pgx

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"myawesomelist.shikanime.studio/internal/config"
)

// NewClientForConfig creates a pgxpool.Pool using DSN information from cfg.
func NewClientForConfig(cfg *config.Config) (*pgxpool.Pool, error) {
	dsnURL, err := cfg.GetDsn()
	if err != nil {
		return nil, err
	}
	if dsnURL.Scheme != "postgres" && dsnURL.Scheme != "postgresql" {
		return nil, err
	}
	return pgxpool.New(context.Background(), dsnURL.String())
}
