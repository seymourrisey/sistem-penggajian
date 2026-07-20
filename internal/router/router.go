package router

import (
	"github.com/gin-gonic/gin"

	"github.com/seymourrisey/sistem-penggajian/internal/handler"
)

func NewRouter(
	departemenHandler *handler.DepartemenHandler,
	karyawanHandler *handler.KaryawanHandler,
	komponenGajiHandler *handler.KomponenGajiHandler,
	payrollHandler *handler.PayrollHandler,
) *gin.Engine {
	r := gin.Default()

	api := r.Group("/api")
	{
		// Departemen
		api.POST("/departemen", departemenHandler.Create)
		api.GET("/departemen", departemenHandler.GetAll)
		api.GET("/departemen/:id", departemenHandler.GetByID)
		api.PUT("/departemen/:id", departemenHandler.Update)
		api.DELETE("/departemen/:id", departemenHandler.Delete)

		// Karyawan
		api.POST("/karyawan", karyawanHandler.Create)
		api.GET("/karyawan", karyawanHandler.GetAll)
		api.GET("/karyawan/:id", karyawanHandler.GetByID)
		api.PUT("/karyawan/:id", karyawanHandler.Update)
		api.DELETE("/karyawan/:id", karyawanHandler.SoftDelete)

		// Komponen Gaji (nested di bawah karyawan)
		api.POST("/karyawan/:id/komponen-gaji", komponenGajiHandler.Create)
		api.GET("/karyawan/:id/komponen-gaji", komponenGajiHandler.GetByKaryawanID)
		api.PUT("/karyawan/:id/komponen-gaji/:komponen_id", komponenGajiHandler.Update)
		api.GET("/komponen-gaji/:id", komponenGajiHandler.GetByID)

		// Payroll
		api.POST("/payroll/generate", payrollHandler.Generate)
		api.GET("/payroll/laporan", payrollHandler.GetLaporan)
		api.GET("/payroll/:karyawan_id/riwayat", payrollHandler.GetRiwayat)
	}

	return r
}
