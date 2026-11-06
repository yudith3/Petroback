package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Connection struct {
	ORM *gorm.DB
	SQL *sql.DB
}

func Open(ctx context.Context, cfg config.DatabaseConfig) (*Connection, error) {
	orm, err := gorm.Open(postgres.Open(cfg.URL()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	sqlDatabase, err := orm.DB()
	if err != nil {
		return nil, fmt.Errorf("access database pool: %w", err)
	}

	sqlDatabase.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDatabase.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDatabase.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	if err := sqlDatabase.PingContext(ctx); err != nil {
		_ = sqlDatabase.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &Connection{ORM: orm, SQL: sqlDatabase}, nil
}

func (c *Connection) Close() error {
	if c == nil || c.SQL == nil {
		return nil
	}
	return c.SQL.Close()
}
