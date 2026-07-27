package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/seymourrisey/sistem-penggajian/internal/model"
	"github.com/seymourrisey/sistem-penggajian/internal/repository"
	"github.com/seymourrisey/sistem-penggajian/internal/service"
)

type DepartemenHandler struct {
	svc service.DepartemenService
}

// NewDepartemenHandler membuat instance DepartemenHandler baru.
func NewDepartemenHandler(svc service.DepartemenService) *DepartemenHandler {
	return &DepartemenHandler{svc: svc}
}

type departemenCreateRequest struct {
	Nama string `json:"nama" binding:"required"`
}

type departemenUpdateRequest struct {
	Nama string `json:"nama" binding:"required"`
}

// Create menangani POST /api/departemen.
func (h *DepartemenHandler) Create(c *gin.Context) {
	var req departemenCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": translateBindError(err)})
		return
	}

	d := &model.Departemen{Nama: req.Nama}
	if err := h.svc.Create(c.Request.Context(), d); err != nil {
		mapDepartemenError(c, err)
		return
	}

	c.JSON(http.StatusCreated, d)
}

// GetAll menangani GET /api/departemen.
func (h *DepartemenHandler) GetAll(c *gin.Context) {
	list, err := h.svc.GetAll(c.Request.Context())
	if err != nil {
		mapDepartemenError(c, err)
		return
	}

	c.JSON(http.StatusOK, list)
}

// GetByID menangani GET /api/departemen/{id}.
func (h *DepartemenHandler) GetByID(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	d, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		mapDepartemenError(c, err)
		return
	}

	c.JSON(http.StatusOK, d)
}

// Update menangani PUT /api/departemen/{id}.
func (h *DepartemenHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req departemenUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": translateBindError(err)})
		return
	}

	dept := &model.Departemen{
		ID:   id,
		Nama: req.Nama,
	}

	if err := h.svc.Update(c.Request.Context(), dept); err != nil {
		mapDepartemenError(c, err)
		return
	}

	c.JSON(http.StatusOK, dept)
}

// Delete menangani DELETE /api/departemen/{id}.
func (h *DepartemenHandler) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		mapDepartemenError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "departemen berhasil di delete"})
}

func mapDepartemenError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repository.ErrDepartemenNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, repository.ErrDepartemenNamaSudahAda):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, repository.ErrDepartemenMasihDipakai):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrDepartemenNamaKosong):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
