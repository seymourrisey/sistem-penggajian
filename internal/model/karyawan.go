package model

import (
	"time"

	"github.com/shopspring/decimal"
)

// StatusKaryawan merepresentasikan nilai kolom status pada tabel karyawan.
type StatusKaryawan string

const (
	StatusKaryawanAktif    StatusKaryawan = "aktif"
	StatusKaryawanNonaktif StatusKaryawan = "nonaktif"
)

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
