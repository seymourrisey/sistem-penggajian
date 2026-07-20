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
	Update(ctx context.Context, k *model.KomponenGaji) error
	GetByID(ctx context.Context, id int) (*model.KomponenGaji, error)
	GetByKaryawanID(ctx context.Context, karyawanID int) ([]model.KomponenGaji, error)
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

func (s *komponenGajiService) GetByID(ctx context.Context, id int) (*model.KomponenGaji, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *komponenGajiService) GetByKaryawanID(ctx context.Context, karyawanID int) ([]model.KomponenGaji, error) {
	return s.repo.GetByKaryawanID(ctx, karyawanID)
}

func (s *komponenGajiService) Update(ctx context.Context, k *model.KomponenGaji) error {
	if strings.TrimSpace(k.Nama) == "" {
		return ErrKomponenNamaKosong
	}
	if k.Jenis != model.JenisKomponenTunjangan && k.Jenis != model.JenisKomponenPotongan {
		return ErrJenisKomponenTidakValid
	}
	return s.repo.Update(ctx, k)
}
