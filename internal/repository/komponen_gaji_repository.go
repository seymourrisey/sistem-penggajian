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

// ErrKomponenGajiNotFound dikembalikan ketika komponen_gaji dengan ID
// tertentu tidak ditemukan.
var ErrKomponenGajiNotFound = errors.New("komponen gaji tidak ditemukan")

// ErrKaryawanTidakValid dikembalikan ketika karyawan_id yang dirujuk tidak
// ada (mapping dari pelanggaran FK constraint
// komponen_gaji.karyawan_id -> karyawan.id).
var ErrKaryawanTidakValid = errors.New("karyawan_id tidak valid atau tidak ditemukan")

// ErrNominalNegatif dikembalikan ketika nominal yang dirujuk tidak
// valid (nominal tidak boleh negatif).
var ErrNominalNegatif = errors.New("nominal tidak boleh negatif")

// ErrKomponenGajiDuplikat dikembalikan ketika kombinasi karyawan_id, jenis,
// dan nama sudah ada (mapping dari pelanggaran UNIQUE constraint
// uq_komponen_gaji_karyawan_jenis_nama).
var ErrKomponenGajiDuplikat = errors.New("komponen gaji dengan jenis dan nama ini sudah ada untuk karyawan tersebut")

// KomponenGajiRepository mendefinisikan kontrak akses data untuk entitas
// KomponenGaji. Sengaja tidak menyediakan Delete — komponen gaji yang sudah
// pernah dipakai dalam generate payroll historis dikoreksi lewat Update
// (mis. HR salah input nominal/persentase), bukan dihapus, untuk menjaga
// jejak data tetap dapat ditelusuri.
type KomponenGajiRepository interface {
	Create(ctx context.Context, k *model.KomponenGaji) error
	GetByID(ctx context.Context, id int) (*model.KomponenGaji, error)
	GetByKaryawanID(ctx context.Context, karyawanID int) ([]model.KomponenGaji, error)
	Update(ctx context.Context, k *model.KomponenGaji) error
	// Delete(ctx context.Context, id int) error
}

// komponenGajiRepository adalah implementasi KomponenGajiRepository
// menggunakan pgx pool.
type komponenGajiRepository struct {
	db *pgxpool.Pool
}

// NewKomponenGajiRepository membuat instance baru KomponenGajiRepository.
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
		if mapped := mapKomponenGajiPgError(err); mapped != nil {
			return mapped
		}
		return fmt.Errorf("gagal membuat komponen gaji untuk karyawan_id=%d: %w", k.KaryawanID, err)
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
		return nil, fmt.Errorf("gagal ambil komponen gaji id=%d: %w", id, err)
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
		return nil, fmt.Errorf("gagal ambil komponen gaji karyawan_id=%d: %w", karyawanID, err)
	}
	defer rows.Close()

	result := make([]model.KomponenGaji, 0)

	for rows.Next() {
		var k model.KomponenGaji
		if err := rows.Scan(
			&k.ID, &k.KaryawanID, &k.Jenis, &k.Nama, &k.Nominal, &k.IsPersen,
		); err != nil {
			return nil, fmt.Errorf("gagal scan komponen gaji karyawan_id=%d: %w", karyawanID, err)
		}
		result = append(result, k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterasi komponen gaji karyawan_id=%d: %w", karyawanID, err)
	}
	return result, nil
}

// Update memperbarui field nama, nominal, is_persen, dan jenis pada record
// komponen_gaji yang sudah ada. karyawan_id tidak diubah lewat method ini.
func (r *komponenGajiRepository) Update(ctx context.Context, k *model.KomponenGaji) error {
	query := `
		UPDATE komponen_gaji
		SET jenis = $1,
		    nama = $2,
		    nominal = $3,
		    is_persen = $4
		WHERE id = $5
		  AND karyawan_id = $6`

	tag, err := r.db.Exec(
		ctx,
		query,
		k.Jenis,
		k.Nama,
		k.Nominal,
		k.IsPersen,
		k.ID,
		k.KaryawanID,
	)
	if err != nil {
		if mapped := mapKomponenGajiPgError(err); mapped != nil {
			return mapped
		}
		return fmt.Errorf("gagal update komponen gaji untuk karyawan_id=%d: %w", k.KaryawanID, err)
	}

	if tag.RowsAffected() == 0 {
		return ErrKomponenGajiNotFound
	}

	return nil
}

// // Delete menghapus permanen record komponen_gaji (hard delete — tabel ini
// func (r *komponenGajiRepository) Delete(ctx context.Context, id int) error {
// 	query := `DELETE FROM komponen_gaji WHERE id = $1`

// 	tag, err := r.db.Exec(ctx, query, id)
// 	if err != nil {
// 		return err
// 	}
// 	if tag.RowsAffected() == 0 {
// 		return ErrKomponenGajiNotFound
// 	}
// 	return nil
// }

// mapKomponenGajiPgError menerjemahkan Postgres error code ke sentinel error
// domain. 23503 = FK karyawan_id invalid (saat Create). 23505 = duplikat
// kombinasi karyawan_id+jenis+nama (saat Create atau Update).
func mapKomponenGajiPgError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23503":
			return ErrKaryawanTidakValid
		case "23505":
			return ErrKomponenGajiDuplikat
		}
	}
	return nil
}
