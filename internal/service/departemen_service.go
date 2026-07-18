// Path: internal/service/departemen_service.go
package service

import (
	"context"
	"errors"
	"strings"

	"github.com/seymourrisey/sistem-penggajian/internal/model"
	"github.com/seymourrisey/sistem-penggajian/internal/repository"
)

// ErrDepartemenNamaKosong dikembalikan ketika nama departemen kosong atau hanya whitespace.
// Divalidasi di service (fail-fast) sebelum menyentuh DB.
var ErrDepartemenNamaKosong = errors.New("nama departemen tidak boleh kosong")

// DepartemenService mendefinisikan business logic untuk entitas Departemen.
type DepartemenService interface {
	Create(ctx context.Context, d *model.Departemen) error
	GetByID(ctx context.Context, id int) (*model.Departemen, error)
	GetAll(ctx context.Context) ([]model.Departemen, error)
	Update(ctx context.Context, d *model.Departemen) error
	Delete(ctx context.Context, id int) error
}

type departemenService struct {
	repo repository.DepartemenRepository
}

// NewDepartemenService membuat instance DepartemenService baru.
// Hanya bergantung pada repository.DepartemenRepository (interface) — tidak
// import driver DB apapun (pgx/pgconn), supaya service mudah di-mock saat
// unit test dan tidak terikat ke implementasi DB tertentu.
func NewDepartemenService(repo repository.DepartemenRepository) DepartemenService {
	return &departemenService{repo: repo}
}

// Create memvalidasi input lalu menyimpan departemen baru.
// Duplikasi nama (repository.ErrDepartemenNamaSudahAda) sudah di-mapping dari
// pelanggaran UNIQUE constraint di dalam repository layer — service tinggal
// meneruskan error tersebut apa adanya ke caller (handler).
func (s *departemenService) Create(ctx context.Context, d *model.Departemen) error {
	if strings.TrimSpace(d.Nama) == "" {
		return ErrDepartemenNamaKosong
	}
	return s.repo.Create(ctx, d)
}

// GetByID mengambil satu departemen berdasarkan ID.
// Error not-found diteruskan apa adanya dari repository (repository.ErrDepartemenNotFound).
func (s *departemenService) GetByID(ctx context.Context, id int) (*model.Departemen, error) {
	return s.repo.GetByID(ctx, id)
}

// GetAll mengambil seluruh daftar departemen.
func (s *departemenService) GetAll(ctx context.Context) ([]model.Departemen, error) {
	return s.repo.GetAll(ctx)
}

// Update memvalidasi input lalu memperbarui data departemen.
func (s *departemenService) Update(ctx context.Context, d *model.Departemen) error {
	if strings.TrimSpace(d.Nama) == "" {
		return ErrDepartemenNamaKosong
	}
	return s.repo.Update(ctx, d)
}

// Delete menghapus departemen berdasarkan ID.
// HARD DELETE
func (s *departemenService) Delete(ctx context.Context, id int) error {
	return s.repo.Delete(ctx, id)
}
