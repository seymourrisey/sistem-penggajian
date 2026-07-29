package model

import (
	"time"

	"github.com/shopspring/decimal"
)

// StatusPayroll merepresentasikan nilai kolom status pada tabel payroll.
type StatusPayroll string

const (
	// StatusPayrollDraft adalah status default saat payroll baru digenerate
	// lewat PayrollService.GeneratePayroll.
	StatusPayrollDraft StatusPayroll = "draft"
	// StatusPayrollFinal menandakan payroll sudah difinalisasi.
	StatusPayrollFinal StatusPayroll = "final"
)

// Payroll merepresentasikan satu baris pada tabel payroll — snapshot hasil
// generate slip gaji satu karyawan untuk satu periode. Nilai GajiPokok,
// TotalTunjangan, TotalPotongan, dan GajiBersih disimpan sebagai snapshot
// (bukan hanya foreign key ke karyawan), supaya perubahan gaji_pokok bulan
// berjalan tidak ikut mengubah riwayat bulan-bulan sebelumnya.
type Payroll struct {
	ID             int             `json:"id" db:"payroll_id"`
	KaryawanID     int             `json:"karyawan_id" db:"karyawan_id"`
	Periode        time.Time       `json:"periode" db:"periode"`
	GajiPokok      decimal.Decimal `json:"gaji_pokok" db:"gaji_pokok"`
	TotalTunjangan decimal.Decimal `json:"total_tunjangan" db:"total_tunjangan"`
	TotalPotongan  decimal.Decimal `json:"total_potongan" db:"total_potongan"`
	GajiBersih     decimal.Decimal `json:"gaji_bersih" db:"gaji_bersih"`
	Status         StatusPayroll   `json:"status" db:"status"`
	CreatedAt      time.Time       `json:"created_at" db:"created_at"`
}
