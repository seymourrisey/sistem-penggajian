package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/seymourrisey/sistem-penggajian/internal/model"
)

var ErrKomponenGajiNotFound = errors.New("komponen gaji not found")

type KomponenGajiRepository interface {
	Create(ctx context.Context, k *model.KomponenGaji) error
	GetByID(ctx context.Context, id int) (*model.KomponenGaji, error)
	GetByKaryawanID(ctx context.Context, karyawanID int) ([]model.KomponenGaji, error)
	Update(ctx context.Context, k *model.KomponenGaji) error
	Delete(ctx context.Context, id int) error
}

type komponenGajiRepository struct {
	db *pgxpool.Pool
}

func NewKomponenGajiRepository(db *pgxpool.Pool) KomponenGajiRepository {
	return &komponenGajiRepository{db: db}
}

// Create menyisipkan record komponen_gaji baru dan mengisi field ID hasil
// generate PostgreSQL (SERIAL) ke struct k.
func (r *komponenGajiRepository) Create(ctx context.Context, k *model.KomponenGaji) error {
	query := `
		INSERT INTO komponen_gaji (karyawan_id, jenis, nama, nominal, is_persen)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`

	err := r.db.QueryRow(ctx, query,
		k.KaryawanID, k.Jenis, k.Nama, k.Nominal, k.IsPersen,
	).Scan(&k.ID)
	if err != nil {
		return err
	}
	return nil
}

// GetByID mengambil satu record komponen_gaji berdasarkan primary key.
// Mengembalikan ErrKomponenGajiNotFound jika tidak ada baris yang cocok.
func (r *komponenGajiRepository) GetByID(ctx context.Context, id int) (*model.KomponenGaji, error) {
	query := `
		SELECT id, karyawan_id, jenis, nama, nominal, is_persen
		FROM komponen_gaji
		WHERE id = $1`

	var k model.KomponenGaji
	err := r.db.QueryRow(ctx, query, id).Scan(
		&k.ID, &k.KaryawanID, &k.Jenis, &k.Nama, &k.Nominal, &k.IsPersen,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrKomponenGajiNotFound
		}
		return nil, err
	}
	return &k, nil
}

// GetByKaryawanID mengambil seluruh komponen_gaji (tunjangan & potongan)
// milik satu karyawan. Digunakan oleh payroll_service saat kalkulasi gaji.
// Mengembalikan slice kosong (bukan error) jika karyawan belum punya komponen.
func (r *komponenGajiRepository) GetByKaryawanID(ctx context.Context, karyawanID int) ([]model.KomponenGaji, error) {
	query := `
		SELECT id, karyawan_id, jenis, nama, nominal, is_persen
		FROM komponen_gaji
		WHERE karyawan_id = $1
		ORDER BY id`

	rows, err := r.db.Query(ctx, query, karyawanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []model.KomponenGaji
	for rows.Next() {
		var k model.KomponenGaji
		if err := rows.Scan(
			&k.ID, &k.KaryawanID, &k.Jenis, &k.Nama, &k.Nominal, &k.IsPersen,
		); err != nil {
			return nil, err
		}
		result = append(result, k)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// Update memperbarui field nama, nominal, is_persen, dan jenis pada record
// komponen_gaji yang sudah ada. karyawan_id tidak diubah lewat method ini.
func (r *komponenGajiRepository) Update(ctx context.Context, k *model.KomponenGaji) error {
	query := `
		UPDATE komponen_gaji
		SET jenis = $1, nama = $2, nominal = $3, is_persen = $4
		WHERE id = $5`

	tag, err := r.db.Exec(ctx, query, k.Jenis, k.Nama, k.Nominal, k.IsPersen, k.ID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrKomponenGajiNotFound
	}
	return nil
}

// Delete menghapus permanen record komponen_gaji (hard delete — tabel ini
// tidak memiliki kolom status, konsisten dengan pola departemen_repository).
func (r *komponenGajiRepository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM komponen_gaji WHERE id = $1`

	tag, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrKomponenGajiNotFound
	}
	return nil
}
