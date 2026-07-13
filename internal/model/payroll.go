package model

import (
	"time"

	"github.com/shopspring/decimal"
)

type StatusPayroll string

const (
	StatusPayrollDraft StatusPayroll = "draft"
	StatusPayrollFinal StatusPayroll = "final"
)

type Payroll struct {
	ID             int             `json:"id" db:"id"`
	KaryawanID     int             `json:"karyawan_id" db:"karyawan_id"`
	Periode        time.Time       `json:"periode" db:"periode"`
	GajiPokok      decimal.Decimal `json:"gaji_pokok" db:"gaji_pokok"`
	TotalTunjangan decimal.Decimal `json:"total_tunjangan" db:"total_tunjangan"`
	TotalPotongan  decimal.Decimal `json:"total_potongan" db:"total_potongan"`
	GajiBersih     decimal.Decimal `json:"gaji_bersih" db:"gaji_bersih"`
	Status         StatusPayroll   `json:"status" db:"status"`
	CreatedAt      time.Time       `json:"created_at" db:"created_at"`
}
