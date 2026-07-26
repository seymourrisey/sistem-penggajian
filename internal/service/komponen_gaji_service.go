// Path: internal/service/komponen_gaji_service.go
package service

import (
	"context"
	"errors"
	"strings"

	"github.com/seymourrisey/sistem-penggajian/internal/model"
	"github.com/seymourrisey/sistem-penggajian/internal/repository"
)

// ErrKomponenNamaKosong dikembalikan ketika nama komponen gaji kosong atau
// hanya whitespace. Divalidasi di service (fail-fast) sebelum menyentuh DB.
var ErrKomponenNamaKosong = errors.New("nama komponen gaji tidak boleh kosong")

// ErrJenisKomponenTidakValid dikembalikan ketika field jenis bukan
// "tunjangan" atau "potongan".
var ErrJenisKomponenTidakValid = errors.New("jenis komponen gaji harus 'tunjangan' atau 'potongan'")

// KomponenGajiService mendefinisikan business logic untuk entitas
// KomponenGaji. Scope method mengikuti 4 endpoint komponen-gaji: tambah,
// list per karyawan, detail, dan koreksi data — sengaja tidak menyediakan Delete
type KomponenGajiService interface {
	Create(ctx context.Context, k *model.KomponenGaji) error
	Update(ctx context.Context, k *model.KomponenGaji) error
	GetByID(ctx context.Context, id int) (*model.KomponenGaji, error)
	GetByKaryawanID(ctx context.Context, karyawanID int) ([]model.KomponenGaji, error)
}

type komponenGajiService struct {
	repo         repository.KomponenGajiRepository
	karyawanRepo repository.KaryawanRepository
}

// NewKomponenGajiService membuat instance KomponenGajiService baru.
func NewKomponenGajiService(
	repo repository.KomponenGajiRepository,
	karyawanRepo repository.KaryawanRepository,
) KomponenGajiService {
	return &komponenGajiService{
		repo:         repo,
		karyawanRepo: karyawanRepo,
	}
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

	if k.Nominal.IsNegative() {
		return repository.ErrNominalNegatif
	}

	karyawan, err := s.karyawanRepo.GetByID(ctx, k.KaryawanID)
	if err != nil {
		return repository.ErrKaryawanTidakValid
	}
	// check status karyawan. status=nonaktif TOLAK
	if karyawan.Status != model.StatusKaryawanAktif {
		return repository.ErrKaryawanTidakAktif
	}

	return s.repo.Create(ctx, k)
}

// GetByID mengambil satu komponen gaji berdasarkan ID.
// Error not-found diteruskan apa adanya dari repository
// (repository.ErrKomponenGajiNotFound).
func (s *komponenGajiService) GetByID(ctx context.Context, id int) (*model.KomponenGaji, error) {
	return s.repo.GetByID(ctx, id)
}

// GetByKaryawanID mengambil seluruh komponen gaji milik satu karyawan.
// Tidak melakukan sorting di layer ini — sorting (F7) dilakukan di handler,
// karena urutan tampilan dianggap presentation concern, bukan business rule.
func (s *komponenGajiService) GetByKaryawanID(ctx context.Context, karyawanID int) ([]model.KomponenGaji, error) {
	return s.repo.GetByKaryawanID(ctx, karyawanID)
}

// Update memvalidasi input lalu memperbarui komponen gaji yang sudah ada.
// Validasi urutan sama dengan Create: nama kosong -> jenis valid -> lalu update.
func (s *komponenGajiService) Update(ctx context.Context, k *model.KomponenGaji) error {
	if strings.TrimSpace(k.Nama) == "" {
		return ErrKomponenNamaKosong
	}
	if k.Jenis != model.JenisKomponenTunjangan && k.Jenis != model.JenisKomponenPotongan {
		return ErrJenisKomponenTidakValid
	}

	if k.Nominal.IsNegative() {
		return repository.ErrNominalNegatif
	}

	karyawan, err := s.karyawanRepo.GetByID(ctx, k.KaryawanID)
	if err != nil {
		return repository.ErrKaryawanTidakValid
	}
	// check status karyawan. status=nonaktif TOLAK
	if karyawan.Status != model.StatusKaryawanAktif {
		return repository.ErrKaryawanTidakAktif
	}

	return s.repo.Update(ctx, k)
}
