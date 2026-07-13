package model

import (
	"github.com/shopspring/decimal"
)

// JenisKomponenGaji merepresentasikan nilai kolom jenis pada tabel komponen_gaji.
type JenisKomponenGaji string

const (
	JenisKomponenTunjangan JenisKomponenGaji = "tunjangan"
	JenisKomponenPotongan  JenisKomponenGaji = "potongan"
)

type KomponenGaji struct {
	ID         int               `json:"id" db:"id"`
	KaryawanID int               `json:"karyawan_id" db:"karyawan_id"`
	Jenis      JenisKomponenGaji `json:"jenis" db:"jenis"`
	Nama       string            `json:"nama" db:"nama"`
	Nominal    decimal.Decimal   `json:"nominal" db:"nominal"`
	IsPersen   bool              `json:"is_persen" db:"is_persen"`
}
