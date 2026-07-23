package main

import (
	"log"
	"net/http/pprof"
	"os"

	"github.com/gin-gonic/gin"

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

	// pprof HANYA di-mount kalau ENABLE_PPROF=true secara eksplisit di
	// environment. Default OFF — mencegah endpoint debug/profiling tak
	// sengaja aktif di production. Dipakai khusus untuk sesi profiling
	// KUK unit kompetensi #7 (docs/profiling-report.md), bukan fitur
	// permanen aplikasi.
	if os.Getenv("ENABLE_PPROF") == "true" {
		registerPprofRoutes(r)
		log.Println("pprof: endpoint /debug/pprof aktif (ENABLE_PPROF=true)")
	}

	log.Printf("server berjalan di port %s", cfg.AppPort)
	if err := r.Run(":" + cfg.AppPort); err != nil {
		log.Fatalf("gagal menjalankan server: %v", err)
	}
}

// registerPprofRoutes mendaftarkan handler net/http/pprof standar ke gin
// Engine lewat gin.WrapF, di bawah prefix /debug/pprof — path yang sama
// dengan konvensi net/http/pprof default, supaya tooling standar
// (go tool pprof http://host/debug/pprof/profile) tetap bisa dipakai
// tanpa konfigurasi tambahan.
func registerPprofRoutes(r *gin.Engine) {
	debug := r.Group("/debug/pprof")
	{
		debug.GET("/", gin.WrapF(pprof.Index))
		debug.GET("/cmdline", gin.WrapF(pprof.Cmdline))
		debug.GET("/profile", gin.WrapF(pprof.Profile))
		debug.POST("/symbol", gin.WrapF(pprof.Symbol))
		debug.GET("/symbol", gin.WrapF(pprof.Symbol))
		debug.GET("/trace", gin.WrapF(pprof.Trace))
		debug.GET("/allocs", gin.WrapF(pprof.Handler("allocs").ServeHTTP))
		debug.GET("/block", gin.WrapF(pprof.Handler("block").ServeHTTP))
		debug.GET("/goroutine", gin.WrapF(pprof.Handler("goroutine").ServeHTTP))
		debug.GET("/heap", gin.WrapF(pprof.Handler("heap").ServeHTTP))
		debug.GET("/mutex", gin.WrapF(pprof.Handler("mutex").ServeHTTP))
		debug.GET("/threadcreate", gin.WrapF(pprof.Handler("threadcreate").ServeHTTP))
	}
}
