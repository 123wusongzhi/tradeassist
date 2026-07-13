package database

import (
	"context"
	"fmt"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/config"
	gmysql "gorm.io/driver/mysql"
	gpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Open returns a GORM DB connection for PostgreSQL or MySQL.
func Open(cfg *config.Config) (*gorm.DB, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	gormCfg := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	}

	var (
		dialector gorm.Dialector
		err       error
	)
	switch cfg.DB.Driver {
	case "mysql":
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			cfg.DB.User, cfg.DB.Password, cfg.DB.Host, cfg.DB.Port, cfg.DB.Name)
		dialector = gmysql.Open(dsn)
	case "postgres":
		password := ""
		if cfg.DB.Password != "" {
			password = fmt.Sprintf(" password=%s", cfg.DB.Password)
		}
		dsn := fmt.Sprintf(
			"host=%s user=%s%s dbname=%s port=%d sslmode=disable TimeZone=%s",
			cfg.DB.Host, cfg.DB.User, password, cfg.DB.Name, cfg.DB.Port, cfg.DB.Timezone,
		)
		dialector = gpostgres.Open(dsn)
	default:
		return nil, fmt.Errorf("unsupported DB_DRIVER %q", cfg.DB.Driver)
	}

	db, err := gorm.Open(dialector, gormCfg)
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	maxOpen := cfg.P7.DBMaxOpenConnections
	if maxOpen < 1 {
		maxOpen = 100
	}
	maxIdle := cfg.P7.DBMaxIdleConnections
	if maxIdle < 0 || maxIdle > maxOpen {
		maxIdle = 10
	}
	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetConnMaxLifetime(durationSeconds(cfg.P7.DBConnMaxLifetimeSeconds, int(time.Hour/time.Second)))
	sqlDB.SetConnMaxIdleTime(durationSeconds(cfg.P7.DBConnMaxIdleTimeSeconds, 900))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("db ping: %w", err)
	}

	return db, nil
}

func durationSeconds(value int, fallback int) time.Duration {
	if value <= 0 {
		value = fallback
	}
	return time.Duration(value) * time.Second
}

// Close releases the underlying sql.DB pool.
func Close(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
