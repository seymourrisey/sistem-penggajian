package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/seymourrisey/sistem-penggajian/internal/config"
)

// NewPool membuat dan memverifikasi koneksi pgxpool.Pool baru berdasarkan
// Config yang diberikan.
//
// Pool dikonfigurasi dengan
//
//	MaxConns=10, MinConns=2,
//	MaxConnLifetime 1 jam, dan
//	MaxConnIdleTime 30 menit persiapan langsung
//
// untuk skalabilitas koneksi (project-design.md NF2).
//
// Melakukan Ping setelah pool dibuat untuk memastikan
// koneksi benar-benar berhasil,
// bukan hanya berhasil di-parse.
func NewPool(cfg *config.Config) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("database: gagal parse DSN: %w", err)
	}

	poolCfg.MaxConns = 10
	poolCfg.MinConns = 2
	poolCfg.MaxConnLifetime = time.Hour
	poolCfg.MaxConnIdleTime = 30 * time.Minute
	poolCfg.HealthCheckPeriod = time.Minute

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("database: gagal membuat connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database: gagal ping database: %w", err)
	}

	return pool, nil
}
