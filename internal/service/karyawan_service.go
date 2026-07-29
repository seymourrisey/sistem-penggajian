package service

import (
	"context"
	"errors"
	"strings"

	"github.com/seymourrisey/sistem-penggajian/internal/model"
	"github.com/seymourrisey/sistem-penggajian/internal/repository"
)

// ErrNipKosong dikembalikan ketika NIP kosong atau hanya whitespace.
var ErrNipKosong = errors.New("nip tidak boleh kosong")

// ErrNamaKaryawanKosong dikembalikan ketika nama karyawan kosong atau hanya whitespace.
var ErrNamaKaryawanKosong = errors.New("nama karyawan tidak boleh kosong")

// ErrGajiPokokNegatif dikembalikan ketika gaji_pokok bernilai negatif.
var ErrGajiPokokNegatif = errors.New("gaji pokok tidak boleh negatif")

// ErrGajiPokokNol dikembalikan ketika gaji_pokok bernilai nol.
var ErrGajiPokokNol = errors.New("gaji pokok tidak boleh nol")

// KaryawanService mendefinisikan business logic untuk entitas Karyawan.
type KaryawanService interface {
	Create(ctx context.Context, k *model.Karyawan) error
	GetByID(ctx context.Context, id int) (*model.Karyawan, error)
	GetAll(ctx context.Context, departemenID *int) ([]model.Karyawan, error)
	Update(ctx context.Context, k *model.Karyawan) error
	SoftDelete(ctx context.Context, id int) error
}

type karyawanService struct {
	repo repository.KaryawanRepository
}

// NewKaryawanService membuat instance KaryawanService baru.
func NewKaryawanService(repo repository.KaryawanRepository) KaryawanService {
	return &karyawanService{repo: repo}
}

// Create memvalidasi input dasar lalu menyimpan karyawan baru.
// Validasi urutan: nip kosong -> nama kosong -> gaji_pokok negatif -> lalu insert.
func (s *karyawanService) Create(ctx context.Context, k *model.Karyawan) error {
	if err := validateKaryawan(k); err != nil {
		return err
	}

	return s.repo.Create(ctx, k)
}

// GetByID mengambil satu karyawan berdasarkan ID.
// Error not-found diteruskan apa adanya dari repository (repository.ErrKaryawanNotFound).
func (s *karyawanService) GetByID(ctx context.Context, id int) (*model.Karyawan, error) {
	return s.repo.GetByID(ctx, id)
}

// GetAll mengambil daftar karyawan, opsional difilter per departemen (F1, F2).
func (s *karyawanService) GetAll(ctx context.Context, departemenID *int) ([]model.Karyawan, error) {
	return s.repo.GetAll(ctx, departemenID)
}

// Update memvalidasi input dasar lalu memperbarui data karyawan.
func (s *karyawanService) Update(ctx context.Context, k *model.Karyawan) error {
	if err := validateKaryawan(k); err != nil {
		return err
	}
	// check status karyawan. status=nonaktif TOLAK
	if k.Status != model.StatusKaryawanAktif {
		return repository.ErrKaryawanTidakAktif
	}
	return s.repo.Update(ctx, k)
}

// SoftDelete mengubah status karyawan menjadi nonaktif bukan HARD DELETE.
// Tidak ada validasi tambahan; diteruskan langsung ke repository.
func (s *karyawanService) SoftDelete(ctx context.Context, id int) error {
	return s.repo.SoftDelete(ctx, id)
}

// validateKaryawan menjalankan validasi dasar yang berlaku untuk Create dan Update.
func validateKaryawan(k *model.Karyawan) error {
	if strings.TrimSpace(k.NIP) == "" {
		return ErrNipKosong
	}
	if strings.TrimSpace(k.Nama) == "" {
		return ErrNamaKaryawanKosong
	}
	if k.GajiPokok.IsNegative() {
		return ErrGajiPokokNegatif
	}
	if k.GajiPokok.IsZero() {
		return ErrGajiPokokNol
	}
	return nil
}
