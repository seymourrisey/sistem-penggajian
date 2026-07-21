package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"github.com/seymourrisey/sistem-penggajian/internal/model"
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

// payrollResponse merepresentasikan body JSON untuk POST&PUT /api/payroll/generate.
// agar 'Periode' konsisten format (YYYY-MM-DD), gunakan string.
type payrollResponse struct {
	ID             int                 `json:"id"`
	KaryawanID     int                 `json:"karyawan_id"`
	Periode        string              `json:"periode"`
	GajiPokok      decimal.Decimal     `json:"gaji_pokok"`
	TotalTunjangan decimal.Decimal     `json:"total_tunjangan"`
	TotalPotongan  decimal.Decimal     `json:"total_potongan"`
	GajiBersih     decimal.Decimal     `json:"gaji_bersih"`
	Status         model.StatusPayroll `json:"status"`
	CreatedAt      time.Time           `json:"created_at"`
}

func newPayrollResponse(p *model.Payroll) payrollResponse {
	return payrollResponse{
		ID:             p.ID,
		KaryawanID:     p.KaryawanID,
		Periode:        p.Periode.Format(dateOnlyLayout),
		GajiPokok:      p.GajiPokok,
		TotalTunjangan: p.TotalTunjangan,
		TotalPotongan:  p.TotalPotongan,
		GajiBersih:     p.GajiBersih,
		Status:         p.Status,
		CreatedAt:      p.CreatedAt,
	}
}

type riwayatResponse struct {
	ID             int                 `json:"id" db:"id"`
	KaryawanID     int                 `json:"karyawan_id" db:"karyawan_id"`
	NIP            string              `json:"nip" db:"nip"`
	NamaKaryawan   string              `json:"nama_karyawan" db:"nama_karyawan"`
	Periode        string              `json:"periode" db:"periode"`
	GajiPokok      decimal.Decimal     `json:"gaji_pokok" db:"gaji_pokok"`
	TotalTunjangan decimal.Decimal     `json:"total_tunjangan" db:"total_tunjangan"`
	TotalPotongan  decimal.Decimal     `json:"total_potongan" db:"total_potongan"`
	GajiBersih     decimal.Decimal     `json:"gaji_bersih" db:"gaji_bersih"`
	Status         model.StatusPayroll `json:"status" db:"status"`
	CreatedAt      time.Time           `json:"created_at" db:"created_at"`
}

func newRiwayatResponse(list []model.PayrollRiwayat) []riwayatResponse {
	resp := make([]riwayatResponse, 0, len(list))

	for _, r := range list {
		resp = append(resp, riwayatResponse{
			ID:             r.ID,
			KaryawanID:     r.KaryawanID,
			NIP:            r.NIP,
			NamaKaryawan:   r.NamaKaryawan,
			Periode:        r.Periode.Format(dateOnlyLayout),
			GajiPokok:      r.GajiPokok,
			TotalTunjangan: r.TotalTunjangan,
			TotalPotongan:  r.TotalPotongan,
			GajiBersih:     r.GajiBersih,
			Status:         r.Status,
			CreatedAt:      r.CreatedAt,
		})
	}

	return resp
}

type laporanResponse struct {
	DepartemenID       int             `json:"departemen_id" db:"departemen_id"`
	NamaDepartemen     string          `json:"nama_departemen" db:"nama_departemen"`
	Periode            string          `json:"periode" db:"periode"`
	JumlahKaryawan     int             `json:"jumlah_karyawan" db:"jumlah_karyawan"`
	TotalGajiBersih    decimal.Decimal `json:"total_gaji_bersih" db:"total_gaji_bersih"`
	RataRataGajiBersih decimal.Decimal `json:"rata_rata_gaji_bersih" db:"rata_rata_gaji_bersih"`
}

func newLaporanResponse(list []model.LaporanDepartemen) []laporanResponse {
	resp := make([]laporanResponse, 0, len(list))

	for _, r := range list {
		resp = append(resp, laporanResponse{
			DepartemenID:       r.DepartemenID,
			NamaDepartemen:     r.NamaDepartemen,
			Periode:            r.Periode.Format(dateOnlyLayout),
			JumlahKaryawan:     r.JumlahKaryawan,
			TotalGajiBersih:    r.TotalGajiBersih,
			RataRataGajiBersih: r.RataRataGajiBersih,
		})
	}

	return resp
}

// Generate menangani POST /api/payroll/generate.
func (h *PayrollHandler) Generate(c *gin.Context) {
	var req payrollGenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": translateBindError(err)})
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

	c.JSON(http.StatusCreated, newPayrollResponse(payroll))
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

	c.JSON(http.StatusOK, newRiwayatResponse(riwayat))
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

	c.JSON(http.StatusOK, newLaporanResponse(laporan))
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
