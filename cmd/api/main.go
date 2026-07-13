package main

import (
	"context"
	"log"

	"github.com/seymourrisey/sistem-penggajian/internal/config"
	"github.com/seymourrisey/sistem-penggajian/pkg/database"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("gagal load config: %v", err)
	}

	pool, err := database.NewPool(cfg)
	if err != nil {
		log.Fatalf("gagal konek database: %v", err)
	}
	defer pool.Close()

	var result int
	if err := pool.QueryRow(context.Background(), "SELECT 1").Scan(&result); err != nil {
		log.Fatalf("query test gagal: %v", err)
	}

	log.Println("database: koneksi berhasil!!")
}
