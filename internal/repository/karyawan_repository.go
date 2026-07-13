package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/seymourrisey/sistem-penggajian/internal/model"
)

// ErrKaryawanNotFound dikembalikan ketika karyawan dengan ID/NIP tertentu tidak ditemukan.
var ErrKaryawanNotFound = errors.New("karyawan tidak ditemukan")

// KaryawanRepository mendefinisikan kontrak akses data untuk entitas Karyawan.
type KaryawanRepository interface {
	Create(ctx context.Context, k *model.Karyawan) error
	GetByID(ctx context.Context, id int) (*model.Karyawan, error)

	// GetAll mengambil daftar karyawan. Jika departemenID != nil, hasil difilter
	// berdasarkan departemen_id tersebut.
	GetAll(ctx context.Context, departemenID *int) ([]model.Karyawan, error)
	Update(ctx context.Context, k *model.Karyawan) error

	// SoftDelete mengubah status karyawan menjadi StatusKaryawanNonaktif (F1).
	SoftDelete(ctx context.Context, id int) error
}

// karyawanRepository adalah implementasi KaryawanRepository menggunakan pgx pool.
type karyawanRepository struct {
	db *pgxpool.Pool
}

// NewKaryawanRepository membuat instance baru KaryawanRepository.
func NewKaryawanRepository(db *pgxpool.Pool) KaryawanRepository {
	return &karyawanRepository{db: db}
}

// Create menambahkan karyawan baru dan mengisi ID, CreatedAt, UpdatedAt hasil insert.
// Status default diambil dari model.StatusKaryawanAktif jika belum diset oleh caller.
func (r *karyawanRepository) Create(ctx context.Context, k *model.Karyawan) error {
	if k.Status == "" {
		k.Status = model.StatusKaryawanAktif
	}

	query := `
		INSERT INTO karyawan (nip, nama, departemen_id, jabatan, gaji_pokok, tanggal_masuk, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at
	`
	err := r.db.QueryRow(ctx, query,
		k.NIP, k.Nama, k.DepartemenID, k.Jabatan, k.GajiPokok, k.TanggalMasuk, k.Status,
	).Scan(&k.ID, &k.CreatedAt, &k.UpdatedAt)
	if err != nil {
		return fmt.Errorf("gagal insert karyawan: %w", err)
	}
	return nil
}

// GetByID mengambil satu karyawan berdasarkan ID.
func (r *karyawanRepository) GetByID(ctx context.Context, id int) (*model.Karyawan, error) {
	query := `
		SELECT id, nip, nama, departemen_id, jabatan, gaji_pokok, tanggal_masuk, status, created_at, updated_at
		FROM karyawan
		WHERE id = $1
	`
	var k model.Karyawan
	err := r.db.QueryRow(ctx, query, id).Scan(
		&k.ID, &k.NIP, &k.Nama, &k.DepartemenID, &k.Jabatan,
		&k.GajiPokok, &k.TanggalMasuk, &k.Status, &k.CreatedAt, &k.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrKaryawanNotFound
		}
		return nil, fmt.Errorf("gagal ambil karyawan id=%d: %w", id, err)
	}
	return &k, nil
}

// GetAll mengambil daftar karyawan, opsional difilter berdasarkan departemen_id.
// Hasil diurutkan berdasarkan nama.
func (r *karyawanRepository) GetAll(ctx context.Context, departemenID *int) ([]model.Karyawan, error) {
	var rows pgx.Rows
	var err error

	baseQuery := `
		SELECT id, nip, nama, departemen_id, jabatan, gaji_pokok, tanggal_masuk, status, created_at, updated_at
		FROM karyawan
	`

	if departemenID != nil {
		query := baseQuery + `WHERE departemen_id = $1 ORDER BY nama ASC`
		rows, err = r.db.Query(ctx, query, *departemenID)
	} else {
		query := baseQuery + `ORDER BY nama ASC`
		rows, err = r.db.Query(ctx, query)
	}
	if err != nil {
		return nil, fmt.Errorf("gagal ambil daftar karyawan: %w", err)
	}
	defer rows.Close()

	var result []model.Karyawan
	for rows.Next() {
		var k model.Karyawan
		if err := rows.Scan(
			&k.ID, &k.NIP, &k.Nama, &k.DepartemenID, &k.Jabatan,
			&k.GajiPokok, &k.TanggalMasuk, &k.Status, &k.CreatedAt, &k.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("gagal scan baris karyawan: %w", err)
		}
		result = append(result, k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error saat iterasi baris karyawan: %w", err)
	}
	return result, nil
}

// Update mengubah data karyawan berdasarkan ID. Kolom updated_at otomatis
// diperbarui ke waktu sekarang dan hasilnya di-scan kembali ke k.UpdatedAt.
func (r *karyawanRepository) Update(ctx context.Context, k *model.Karyawan) error {
	query := `
		UPDATE karyawan
		SET nip = $1,
		    nama = $2,
		    departemen_id = $3,
		    jabatan = $4,
		    gaji_pokok = $5,
		    tanggal_masuk = $6,
		    status = $7,
		    updated_at = NOW()
		WHERE id = $8
		RETURNING updated_at
	`
	err := r.db.QueryRow(ctx, query,
		k.NIP, k.Nama, k.DepartemenID, k.Jabatan, k.GajiPokok, k.TanggalMasuk, k.Status, k.ID,
	).Scan(&k.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrKaryawanNotFound
		}
		return fmt.Errorf("gagal update karyawan id=%d: %w", k.ID, err)
	}
	return nil
}

// SoftDelete mengubah status karyawan menjadi nonaktif (F1: soft-delete).
// Tidak menghapus baris dari database — hanya mengubah kolom status.
func (r *karyawanRepository) SoftDelete(ctx context.Context, id int) error {
	query := `
		UPDATE karyawan
		SET status = $1, updated_at = NOW()
		WHERE id = $2
	`
	cmdTag, err := r.db.Exec(ctx, query, model.StatusKaryawanNonaktif, id)
	if err != nil {
		return fmt.Errorf("gagal soft-delete karyawan id=%d: %w", id, err)
	}
	if cmdTag.RowsAffected() == 0 {
		return ErrKaryawanNotFound
	}
	return nil
}
