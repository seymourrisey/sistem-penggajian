package model

import (
	"time"

	"github.com/shopspring/decimal"
)

// PayrollRiwayat merepresentasikan satu baris riwayat gaji karyawan, hasil
// JOIN antara payroll dan karyawan (untuk menampilkan nip & nama tanpa
// query terpisah). Digunakan oleh GET /api/payroll/:karyawan_id/riwayat.
type PayrollRiwayat struct {
	ID             int             `json:"id" db:"id"`
	KaryawanID     int             `json:"karyawan_id" db:"karyawan_id"`
	NIP            string          `json:"nip" db:"nip"`
	NamaKaryawan   string          `json:"nama_karyawan" db:"nama_karyawan"`
	Periode        time.Time       `json:"periode" db:"periode"`
	GajiPokok      decimal.Decimal `json:"gaji_pokok" db:"gaji_pokok"`
	TotalTunjangan decimal.Decimal `json:"total_tunjangan" db:"total_tunjangan"`
	TotalPotongan  decimal.Decimal `json:"total_potongan" db:"total_potongan"`
	GajiBersih     decimal.Decimal `json:"gaji_bersih" db:"gaji_bersih"`
	Status         StatusPayroll   `json:"status" db:"status"`
	CreatedAt      time.Time       `json:"created_at" db:"created_at"`
}

// LaporanDepartemen merepresentasikan hasil agregat gaji per departemen
// untuk satu periode (F6) — JOIN payroll + karyawan + departemen, GROUP BY
// departemen, dengan SUM dan AVG atas gaji_bersih.
type LaporanDepartemen struct {
	DepartemenID       int             `json:"departemen_id" db:"departemen_id"`
	NamaDepartemen     string          `json:"nama_departemen" db:"nama_departemen"`
	Periode            time.Time       `json:"periode" db:"periode"`
	JumlahKaryawan     int             `json:"jumlah_karyawan" db:"jumlah_karyawan"`
	TotalGajiBersih    decimal.Decimal `json:"total_gaji_bersih" db:"total_gaji_bersih"`
	RataRataGajiBersih decimal.Decimal `json:"rata_rata_gaji_bersih" db:"rata_rata_gaji_bersih"`
}
