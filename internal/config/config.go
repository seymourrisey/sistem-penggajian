package config

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Config menyimpan seluruh nilai konfigurasi aplikasi yang dibaca dari
// environment variable (lewat file .env atau environment asli), meliputi
// kredensial koneksi database dan port HTTP server.
type Config struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	AppPort    string
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// DSN mengembalikan connection string PostgreSQL yang siap dipakai oleh pgx.
func (c *Config) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName,
	)
}

// LoadConfig membaca file .env (jika ada) lalu memuat seluruh environment
// variable yang dibutuhkan aplikasi ke dalam Config. DB_USER dan DB_NAME
// wajib diisi — jika salah satu kosong, LoadConfig mengembalikan error
// alih-alih Config dengan nilai default yang tidak aman dipakai.
func LoadConfig() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		log.Println("INFO: file.env tidak ditemukan!!")
	}

	cfg := &Config{
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", ""),
		DBPassword: getEnv("DB_PASSWORD", ""),
		DBName:     getEnv("DB_NAME", ""),
		AppPort:    getEnv("APP_PORT", "8080"),
	}

	if cfg.DBUser == "" || cfg.DBName == "" {
		return nil, fmt.Errorf("config: DB_USER dan DB_NAME wajib diisi (cek file .env)")
	}

	return cfg, nil
}
