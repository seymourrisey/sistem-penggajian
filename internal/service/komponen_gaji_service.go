// Path: internal/service/komponen_gaji_service.go
package service

import (
	"context"
	"errors"
	"strings"

	"github.com/seymourrisey/sistem-penggajian/internal/model"
	"github.com/seymourrisey/sistem-penggajian/internal/repository"
)

var ErrKomponenNamaKosong = errors.New("nama komponen gaji tidak boleh kosong")
var ErrJenisKomponenTidakValid = errors.New("jenis komponen gaji harus 'tunjangan' atau 'potongan'")

type KomponenGajiService interface {
	Create(ctx context.Context, k *model.KomponenGaji) error
}

type komponenGajiService struct {
	repo repository.KomponenGajiRepository
}

// NewKomponenGajiService membuat instance KomponenGajiService baru.
func NewKomponenGajiService(repo repository.KomponenGajiRepository) KomponenGajiService {
	return &komponenGajiService{repo: repo}
}

// Create memvalidasi input lalu menyimpan komponen gaji baru.
// Validasi urutan: nama kosong -> jenis valid -> lalu insert.
func (s *komponenGajiService) Create(ctx context.Context, k *model.KomponenGaji) error {
	if strings.TrimSpace(k.Nama) == "" {
		return ErrKomponenNamaKosong
	}
	if k.Jenis != model.JenisKomponenTunjangan && k.Jenis != model.JenisKomponenPotongan {
		return ErrJenisKomponenTidakValid
	}
	return s.repo.Create(ctx, k)
}
