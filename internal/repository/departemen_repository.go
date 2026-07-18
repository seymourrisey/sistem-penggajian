package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/seymourrisey/sistem-penggajian/internal/model"
)

// ErrDepartemenNotFound dikembalikan ketika departemen dengan ID tertentu tidak ditemukan.
var ErrDepartemenNotFound = errors.New("departemen tidak ditemukan")

// ErrDepartemenNamaSudahAda dikembalikan ketika nama departemen sudah terdaftar
// (mapping dari pelanggaran UNIQUE constraint pada kolom nama).
var ErrDepartemenNamaSudahAda = errors.New("nama departemen sudah terdaftar")

// DepartemenRepository mendefinisikan kontrak akses data untuk entitas Departemen.
type DepartemenRepository interface {
	Create(ctx context.Context, d *model.Departemen) error
	GetByID(ctx context.Context, id int) (*model.Departemen, error)
	GetAll(ctx context.Context) ([]model.Departemen, error)
	Update(ctx context.Context, d *model.Departemen) error
	Delete(ctx context.Context, id int) error
}

// departemenRepository adalah implementasi DepartemenRepository menggunakan pgx pool.
type departemenRepository struct {
	db *pgxpool.Pool
}

// NewDepartemenRepository membuat instance baru DepartemenRepository.
func NewDepartemenRepository(db *pgxpool.Pool) DepartemenRepository {
	return &departemenRepository{db: db}
}

// Create menambahkan departemen baru dan mengisi ID serta CreatedAt hasil insert.
func (r *departemenRepository) Create(ctx context.Context, d *model.Departemen) error {
	query := `
		INSERT INTO departemen (nama)
		VALUES ($1)
		RETURNING id, created_at
	`
	err := r.db.QueryRow(ctx, query, d.Nama).Scan(&d.ID, &d.CreatedAt)
	if err != nil {
		if mapped := mapDepartemenPgError(err); mapped != nil {
			return mapped
		}
		return fmt.Errorf("gagal insert departemen: %w", err)
	}
	return nil
}

// GetByID mengambil satu departemen berdasarkan ID.
func (r *departemenRepository) GetByID(ctx context.Context, id int) (*model.Departemen, error) {
	query := `
		SELECT id, nama, created_at
		FROM departemen
		WHERE id = $1
	`
	var d model.Departemen
	err := r.db.QueryRow(ctx, query, id).Scan(&d.ID, &d.Nama, &d.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrDepartemenNotFound
		}
		return nil, fmt.Errorf("gagal ambil departemen id=%d: %w", id, err)
	}
	return &d, nil
}

// GetAll mengambil seluruh data departemen, diurutkan berdasarkan nama.
func (r *departemenRepository) GetAll(ctx context.Context) ([]model.Departemen, error) {
	query := `
		SELECT id, nama, created_at
		FROM departemen
		ORDER BY nama ASC
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("gagal ambil daftar departemen: %w", err)
	}
	defer rows.Close()
	var result []model.Departemen
	for rows.Next() {
		var d model.Departemen
		if err := rows.Scan(&d.ID, &d.Nama, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("gagal scan baris departemen: %w", err)
		}
		result = append(result, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error saat iterasi baris departemen: %w", err)
	}
	return result, nil
}

// Update mengubah nama departemen berdasarkan ID.
func (r *departemenRepository) Update(ctx context.Context, d *model.Departemen) error {
	query := `
		UPDATE departemen
		SET nama = $1
		WHERE id = $2
	`
	cmdTag, err := r.db.Exec(ctx, query, d.Nama, d.ID)
	if err != nil {
		if mapped := mapDepartemenPgError(err); mapped != nil {
			return mapped
		}
		return fmt.Errorf("gagal update departemen id=%d: %w", d.ID, err)
	}
	if cmdTag.RowsAffected() == 0 {
		return ErrDepartemenNotFound
	}
	return nil
}

// Delete menghapus departemen berdasarkan ID (hard delete — tabel departemen
// tidak memiliki kolom status, berbeda dengan karyawan yang pakai soft-delete).
func (r *departemenRepository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM departemen WHERE id = $1`
	cmdTag, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("gagal hapus departemen id=%d: %w", id, err)
	}
	if cmdTag.RowsAffected() == 0 {
		return ErrDepartemenNotFound
	}
	return nil
}

// mapDepartemenPgError menerjemahkan Postgres error code 23505 (unique violation
// pada kolom nama) menjadi ErrDepartemenNamaSudahAda. Mengembalikan nil jika
// error bukan pelanggaran constraint yang dikenali, supaya caller tetap
// membungkusnya dengan fmt.Errorf seperti sebelumnya.
func mapDepartemenPgError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrDepartemenNamaSudahAda
	}
	return nil
}
