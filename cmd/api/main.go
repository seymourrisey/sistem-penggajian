package main

import (
	"log"

	"github.com/seymourrisey/sistem-penggajian/internal/config"
	"github.com/seymourrisey/sistem-penggajian/internal/handler"
	"github.com/seymourrisey/sistem-penggajian/internal/repository"
	"github.com/seymourrisey/sistem-penggajian/internal/router"
	"github.com/seymourrisey/sistem-penggajian/internal/service"
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
	log.Println("database: koneksi berhasil")

	// Repository layer
	departemenRepo := repository.NewDepartemenRepository(pool)
	karyawanRepo := repository.NewKaryawanRepository(pool)
	komponenGajiRepo := repository.NewKomponenGajiRepository(pool)
	payrollRepo := repository.NewPayrollRepository(pool)

	// Service layer
	departemenSvc := service.NewDepartemenService(departemenRepo)
	karyawanSvc := service.NewKaryawanService(karyawanRepo)
	komponenGajiSvc := service.NewKomponenGajiService(komponenGajiRepo)
	payrollSvc := service.NewPayrollService(karyawanRepo, komponenGajiRepo, payrollRepo)

	// Handler layer
	departemenHandler := handler.NewDepartemenHandler(departemenSvc)
	karyawanHandler := handler.NewKaryawanHandler(karyawanSvc)
	komponenGajiHandler := handler.NewKomponenGajiHandler(komponenGajiSvc)
	payrollHandler := handler.NewPayrollHandler(payrollSvc)

	// Router
	r := router.NewRouter(
		departemenHandler,
		karyawanHandler,
		komponenGajiHandler,
		payrollHandler)

	log.Printf("server berjalan di port %s", cfg.AppPort)
	if err := r.Run(":" + cfg.AppPort); err != nil {
		log.Fatalf("gagal menjalankan server: %v", err)
	}
}
