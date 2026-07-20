package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/seymourrisey/sistem-penggajian/internal/repository"
	"github.com/seymourrisey/sistem-penggajian/internal/service"
)

// PayrollHandler menangani HTTP request untuk entitas Payroll.
type PayrollHandler struct {
	svc service.PayrollService
}

// NewPayrollHandler membuat instance PayrollHandler baru.
func NewPayrollHandler(svc service.PayrollService) *PayrollHandler {
	return &PayrollHandler{svc: svc}
}

// payrollGenerateRequest merepresentasikan body JSON untuk POST /api/payroll/generate.
// karyawan_id dan periode keduanya di body (disepakati di awal room).
type payrollGenerateRequest struct {
	KaryawanID int    `json:"karyawan_id" binding:"required"`
	Periode    string `json:"periode" binding:"required"`
}

// Generate menangani POST /api/payroll/generate.
func (h *PayrollHandler) Generate(c *gin.Context) {
	var req payrollGenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	periode, err := time.Parse(dateOnlyLayout, req.Periode)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "periode harus format YYYY-MM-DD"})
		return
	}

	payroll, err := h.svc.GeneratePayroll(c.Request.Context(), req.KaryawanID, periode)
	if err != nil {
		mapPayrollError(c, err)
		return
	}

	c.JSON(http.StatusCreated, payroll)
}

// GetRiwayat menangani GET /api/payroll/:karyawan_id/riwayat.
func (h *PayrollHandler) GetRiwayat(c *gin.Context) {
	karyawanID, err := strconv.Atoi(c.Param("karyawan_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "karyawan_id harus berupa angka"})
		return
	}

	riwayat, err := h.svc.GetRiwayat(c.Request.Context(), karyawanID)
	if err != nil {
		mapPayrollError(c, err)
		return
	}

	c.JSON(http.StatusOK, riwayat)
}

// GetLaporan menangani GET /api/payroll/laporan?periode=YYYY-MM-DD.
func (h *PayrollHandler) GetLaporan(c *gin.Context) {
	raw := c.Query("periode")
	if raw == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query param periode wajib diisi (format YYYY-MM-DD)"})
		return
	}

	periode, err := time.Parse(dateOnlyLayout, raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "periode harus format YYYY-MM-DD"})
		return
	}

	laporan, err := h.svc.GetLaporan(c.Request.Context(), periode)
	if err != nil {
		mapPayrollError(c, err)
		return
	}

	c.JSON(http.StatusOK, laporan)
}

// mapPayrollError memetakan error dari service/repository layer ke HTTP
// status code, mengikuti skema yang sama dengan handler lain:
//   - not found            -> 404
//   - duplikat / conflict  -> 409
//   - selain itu           -> 500
func mapPayrollError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repository.ErrKaryawanNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, repository.ErrPayrollAlreadyExists):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
