package model

import (
	"time"

	"github.com/shopspring/decimal"
)

// StatusKaryawan merepresentasikan nilai kolom status pada tabel karyawan.
type StatusKaryawan string

const (
	// StatusKaryawanAktif menandakan karyawan masih aktif bekerja.
	StatusKaryawanAktif StatusKaryawan = "aktif"
	// StatusKaryawanNonaktif menandakan karyawan sudah dinonaktifkan lewat
	// soft-delete (KaryawanRepository.SoftDelete) — baris data tetap
	// tersimpan untuk menjaga audit trail riwayat payroll.
	StatusKaryawanNonaktif StatusKaryawan = "nonaktif"
)

// Karyawan merepresentasikan satu baris pada tabel karyawan. Field Status
// sengaja hanya boleh berubah lewat jalur SoftDelete (state transition),
// bukan lewat Update data biasa (nip/nama/jabatan/gaji_pokok)
type Karyawan struct {
	ID           int             `json:"id" db:"id"`
	NIP          string          `json:"nip" db:"nip"`
	Nama         string          `json:"nama" db:"nama"`
	DepartemenID int             `json:"departemen_id" db:"departemen_id"`
	Jabatan      string          `json:"jabatan" db:"jabatan"`
	GajiPokok    decimal.Decimal `json:"gaji_pokok" db:"gaji_pokok"`
	TanggalMasuk time.Time       `json:"tanggal_masuk" db:"tanggal_masuk"`
	Status       StatusKaryawan  `json:"status" db:"status"`
	CreatedAt    time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at" db:"updated_at"`
}
