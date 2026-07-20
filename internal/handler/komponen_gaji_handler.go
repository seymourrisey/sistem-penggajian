package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"github.com/seymourrisey/sistem-penggajian/internal/model"
	"github.com/seymourrisey/sistem-penggajian/internal/repository"
	"github.com/seymourrisey/sistem-penggajian/internal/service"
)

type KomponenGajiHandler struct {
	svc service.KomponenGajiService
}

func NewKomponenGajiHandler(svc service.KomponenGajiService) *KomponenGajiHandler {
	return &KomponenGajiHandler{svc: svc}
}

// komponenGajiCreateRequest merepresentasikan body JSON untuk
// POST /api/karyawan/:id/komponen-gaji. karyawan_id diambil dari URL param,
// bukan dari body.
type komponenGajiCreateRequest struct {
	Jenis    string          `json:"jenis" binding:"required"`
	Nama     string          `json:"nama" binding:"required"`
	Nominal  decimal.Decimal `json:"nominal" binding:"required"`
	IsPersen bool            `json:"is_persen"`
}

// Create menangani POST /api/karyawan/:id/komponen-gaji.
func (h *KomponenGajiHandler) Create(c *gin.Context) {
	karyawanID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id harus berupa angka"})
		return
	}

	var req komponenGajiCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	k := &model.KomponenGaji{
		KaryawanID: karyawanID,
		Jenis:      model.JenisKomponenGaji(req.Jenis),
		Nama:       req.Nama,
		Nominal:    req.Nominal,
		IsPersen:   req.IsPersen,
	}

	if err := h.svc.Create(c.Request.Context(), k); err != nil {
		mapKomponenGajiError(c, err)
		return
	}

	c.JSON(http.StatusCreated, k)
}

// mapKomponenGajiError memetakan error dari service/repository layer ke HTTP
func mapKomponenGajiError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repository.ErrKaryawanTidakValid):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrKomponenNamaKosong),
		errors.Is(err, service.ErrJenisKomponenTidakValid):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
