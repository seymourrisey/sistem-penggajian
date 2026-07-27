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

// dateOnlyLayout adalah format tanggal yang diterima/dikirim di JSON body
// untuk field tanggal (tanggal_masuk, periode). Disepakati: "YYYY-MM-DD".
const dateOnlyLayout = "2006-01-02"

// KaryawanHandler menangani HTTP request untuk entitas Karyawan.
type KaryawanHandler struct {
	svc service.KaryawanService
}

// NewKaryawanHandler membuat instance KaryawanHandler baru.
func NewKaryawanHandler(svc service.KaryawanService) *KaryawanHandler {
	return &KaryawanHandler{svc: svc}
}

// karyawanCreateRequest merepresentasikan body JSON untuk POST /api/karyawan.
type karyawanCreateRequest struct {
	NIP          string          `json:"nip" binding:"required"`
	Nama         string          `json:"nama" binding:"required"`
	DepartemenID int             `json:"departemen_id" binding:"required"`
	Jabatan      string          `json:"jabatan" binding:"required"`
	GajiPokok    decimal.Decimal `json:"gaji_pokok" binding:"required"`
	TanggalMasuk string          `json:"tanggal_masuk" binding:"required"`
}

// karyawanUpdateRequest merepresentasikan body JSON untuk PUT /api/karyawan/:id.
type karyawanUpdateRequest struct {
	NIP          string          `json:"nip" binding:"required"`
	Nama         string          `json:"nama" binding:"required"`
	DepartemenID int             `json:"departemen_id" binding:"required"`
	Jabatan      string          `json:"jabatan" binding:"required"`
	GajiPokok    decimal.Decimal `json:"gaji_pokok" binding:"required"`
	TanggalMasuk string          `json:"tanggal_masuk" binding:"required"`
}

// karyawanResponse merepresentasikan body JSON untuk GET /api/karyawan/:id.
type karyawanResponse struct {
	ID           int                  `json:"id" db:"id"`
	NIP          string               `json:"nip" db:"nip"`
	Nama         string               `json:"nama" db:"nama"`
	DepartemenID int                  `json:"departemen_id" db:"departemen_id"`
	Jabatan      string               `json:"jabatan" db:"jabatan"`
	GajiPokok    decimal.Decimal      `json:"gaji_pokok" db:"gaji_pokok"`
	TanggalMasuk string               `json:"tanggal_masuk" db:"tanggal_masuk"`
	Status       model.StatusKaryawan `json:"status" db:"status"`
	CreatedAt    time.Time            `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time            `json:"updated_at" db:"updated_at"`
}

// newKaryawanResponse mengembalikan karyawanResponse dari model.Karyawan.
// TanggalMasuk di-format sebagai string dalam format YYYY-MM-DD.
func newKaryawanResponse(k *model.Karyawan) karyawanResponse {
	return karyawanResponse{
		ID:           k.ID,
		NIP:          k.NIP,
		Nama:         k.Nama,
		DepartemenID: k.DepartemenID,
		Jabatan:      k.Jabatan,
		GajiPokok:    k.GajiPokok,
		TanggalMasuk: k.TanggalMasuk.Format(dateOnlyLayout),
		Status:       k.Status,
		CreatedAt:    k.CreatedAt,
		UpdatedAt:    k.UpdatedAt,
	}
}

// listKaryawanResponse mengembalikan slice dari karyawanResponse dari slice model.Karyawan.
// TanggalMasuk di-format sebagai string dalam format YYYY-MM-DD.
func listKaryawanResponse(list []model.Karyawan) []karyawanResponse {
	resp := make([]karyawanResponse, 0, len(list))
	for _, k := range list {
		resp = append(resp, karyawanResponse{
			ID:           k.ID,
			NIP:          k.NIP,
			Nama:         k.Nama,
			DepartemenID: k.DepartemenID,
			Jabatan:      k.Jabatan,
			GajiPokok:    k.GajiPokok,
			TanggalMasuk: k.TanggalMasuk.Format(dateOnlyLayout),
			Status:       k.Status,
			CreatedAt:    k.CreatedAt,
			UpdatedAt:    k.UpdatedAt,
		})
	}
	return resp
}

// Create menangani POST /api/karyawan.
func (h *KaryawanHandler) Create(c *gin.Context) {
	var req karyawanCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": translateBindError(err)})
		return
	}

	tanggalMasuk, err := time.Parse(dateOnlyLayout, req.TanggalMasuk)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tanggal_masuk harus format YYYY-MM-DD"})
		return
	}

	k := &model.Karyawan{
		NIP:          req.NIP,
		Nama:         req.Nama,
		DepartemenID: req.DepartemenID,
		Jabatan:      req.Jabatan,
		GajiPokok:    req.GajiPokok,
		TanggalMasuk: tanggalMasuk,
	}

	if err := h.svc.Create(c.Request.Context(), k); err != nil {
		mapKaryawanError(c, err)
		return
	}

	c.JSON(http.StatusCreated, newKaryawanResponse(k))
}

// GetAll menangani GET /api/karyawan, dengan filter opsional ?departemen=.
func (h *KaryawanHandler) GetAll(c *gin.Context) {
	var departemenID *int
	if raw := c.Query("departemen"); raw != "" {
		id, err := strconv.Atoi(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "id departemen harus berupa angka"})
			return
		}
		departemenID = &id
	}

	list, err := h.svc.GetAll(c.Request.Context(), departemenID)
	if err != nil {
		mapKaryawanError(c, err)
		return
	}

	c.JSON(http.StatusOK, listKaryawanResponse(list))
}

// GetByID menangani GET /api/karyawan/:id.
func (h *KaryawanHandler) GetByID(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id harus berupa angka"})
		return
	}

	k, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		mapKaryawanError(c, err)
		return
	}

	c.JSON(http.StatusOK, newKaryawanResponse(k))
}

// Update menangani PUT /api/karyawan/:id.
func (h *KaryawanHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id harus berupa angka"})
		return
	}

	var req karyawanUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": translateBindError(err)})
		return
	}

	tanggalMasuk, err := time.Parse(dateOnlyLayout, req.TanggalMasuk)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tanggal_masuk harus format YYYY-MM-DD"})
		return
	}

	k := &model.Karyawan{
		ID:           id,
		NIP:          req.NIP,
		Nama:         req.Nama,
		DepartemenID: req.DepartemenID,
		Jabatan:      req.Jabatan,
		GajiPokok:    req.GajiPokok,
		TanggalMasuk: tanggalMasuk,
	}

	if err := h.svc.Update(c.Request.Context(), k); err != nil {
		mapKaryawanError(c, err)
		return
	}

	c.JSON(http.StatusOK, newKaryawanResponse(k))
}

// SoftDelete menangani DELETE /api/karyawan/:id (ubah status jadi nonaktif).
func (h *KaryawanHandler) SoftDelete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id harus berupa angka"})
		return
	}

	if err := h.svc.SoftDelete(c.Request.Context(), id); err != nil {
		mapKaryawanError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "karyawan berhasil dinonaktifkan"})
}

func mapKaryawanError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repository.ErrKaryawanNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, repository.ErrNipSudahAda):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, repository.ErrDepartemenTidakValid):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrNipKosong),
		errors.Is(err, service.ErrNamaKaryawanKosong),
		errors.Is(err, service.ErrGajiPokokNegatif),
		errors.Is(err, service.ErrGajiPokokNol):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
