package model

import (
	"github.com/shopspring/decimal"
)

// JenisKomponenGaji merepresentasikan nilai kolom jenis pada tabel komponen_gaji.
type JenisKomponenGaji string

const (
	// JenisKomponenTunjangan menandakan komponen menambah gaji_bersih saat
	// generate payroll.
	JenisKomponenTunjangan JenisKomponenGaji = "tunjangan"
	// JenisKomponenPotongan menandakan komponen mengurangi gaji_bersih saat
	// generate payroll.
	JenisKomponenPotongan JenisKomponenGaji = "potongan"
)

// KomponenGaji merepresentasikan satu baris pada tabel komponen_gaji —
// tunjangan atau potongan milik satu karyawan. Jika IsPersen bernilai true,
// Nominal diinterpretasikan sebagai persentase dari gaji_pokok karyawan
// (basis selalu gaji_pokok, bukan berjenjang); jika false, Nominal adalah
// nilai rupiah flat. Lihat PayrollService.GeneratePayroll untuk logic kalkulasi.
type KomponenGaji struct {
	ID         int               `json:"id" db:"id"`
	KaryawanID int               `json:"karyawan_id" db:"karyawan_id"`
	Jenis      JenisKomponenGaji `json:"jenis" db:"jenis"`
	Nama       string            `json:"nama" db:"nama"`
	Nominal    decimal.Decimal   `json:"nominal" db:"nominal"`
	IsPersen   bool              `json:"is_persen" db:"is_persen"`
}
