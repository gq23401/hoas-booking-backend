package postgres

import (
	"fmt"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/yourusername/hoas-booking/config"
)

func NewDB(cfg config.DatabaseConfig) (*sqlx.DB, error) {
	db, err := sqlx.Open("postgres", cfg.DSN())
	if err != nil { return nil, fmt.Errorf("open postgres: %w", err) }
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	if err := db.Ping(); err != nil { return nil, fmt.Errorf("ping postgres: %w", err) }
	return db, nil
}

func toNullableTime(t interface{}) interface{} { return t }
