package service

import (
	"context"
	"time"

	"github.com/shopspring/decimal"

	"github.com/seymourrisey/sistem-penggajian/internal/model"
	"github.com/seymourrisey/sistem-penggajian/internal/repository"
)

// seratus digunakan sebagai pembagi saat menghitung komponen gaji berbasis
// persentase (is_persen = true).
var seratus = decimal.NewFromInt(100)

type PayrollService interface {
	GeneratePayroll(ctx context.Context, karyawanID int, periode time.Time) (*model.Payroll, error)
}

type payrollService struct {
	karyawanRepo repository.KaryawanRepository
	komponenRepo repository.KomponenGajiRepository
	payrollRepo  repository.PayrollRepository
}

func NewPayrollService(
	karyawanRepo repository.KaryawanRepository,
	komponenRepo repository.KomponenGajiRepository,
	payrollRepo repository.PayrollRepository,
) PayrollService {
	return &payrollService{
		karyawanRepo: karyawanRepo,
		komponenRepo: komponenRepo,
		payrollRepo:  payrollRepo,
	}
}

// GeneratePayroll menghitung dan menyimpan slip gaji satu karyawan untuk satu
// periode (F4). Algoritma:
//  1. Ambil gaji_pokok karyawan.
//  2. Ambil seluruh komponen_gaji milik karyawan tsb.
//  3. Loop komponen: jika is_persen true, nilai = gaji_pokok * nominal / 100;
//     jika false, nilai = nominal apa adanya. Basis persentase SELALU
//     gaji_pokok (flat basis, bukan berjenjang) — keputusan bisnis eksplisit.
//  4. Sum nilai per jenis -> total_tunjangan, total_potongan.
//  5. gaji_bersih = gaji_pokok + total_tunjangan - total_potongan.
//  6. Simpan sebagai record payroll baru berstatus "draft".
//
// Pemetaan error: karyawan tidak ditemukan -> repository.ErrKaryawanNotFound;
// kombinasi karyawan_id+periode sudah pernah digenerate ->
// repository.ErrPayrollAlreadyExists (dipetakan di PayrollRepository.Create
// dari constraint UNIQUE, tidak ada query cek terpisah di layer ini).
// Fungsi ini sengaja tidak menyentuh HTTP sama sekali agar mudah diuji
// terpisah lewat mock repository (kompetensi #9).
func (s *payrollService) GeneratePayroll(ctx context.Context, karyawanID int, periode time.Time) (*model.Payroll, error) {
	karyawan, err := s.karyawanRepo.GetByID(ctx, karyawanID)
	if err != nil {
		return nil, err
	}

	komponenList, err := s.komponenRepo.GetByKaryawanID(ctx, karyawanID)
	if err != nil {
		return nil, err
	}

	totalTunjangan := decimal.Zero
	totalPotongan := decimal.Zero

	for _, k := range komponenList {
		nilai := hitungNilaiKomponen(karyawan.GajiPokok, k)

		switch k.Jenis {
		case model.JenisKomponenTunjangan:
			totalTunjangan = totalTunjangan.Add(nilai)
		case model.JenisKomponenPotongan:
			totalPotongan = totalPotongan.Add(nilai)
		}
	}

	gajiBersih := karyawan.GajiPokok.Add(totalTunjangan).Sub(totalPotongan)

	payroll := &model.Payroll{
		KaryawanID:     karyawanID,
		Periode:        periode,
		GajiPokok:      karyawan.GajiPokok,
		TotalTunjangan: totalTunjangan,
		TotalPotongan:  totalPotongan,
		GajiBersih:     gajiBersih,
		Status:         model.StatusPayrollDraft,
	}

	if err := s.payrollRepo.Create(ctx, payroll); err != nil {
		return nil, err
	}

	return payroll, nil
}

// hitungNilaiKomponen menghitung nilai rupiah satu komponen_gaji terhadap
// gaji_pokok. Dipisah jadi fungsi murni (tanpa DB/ctx) agar mudah diuji
// langsung tanpa mock — mencakup skenario is_persen true/false.
func hitungNilaiKomponen(gajiPokok decimal.Decimal, k model.KomponenGaji) decimal.Decimal {
	if k.IsPersen {
		return gajiPokok.Mul(k.Nominal).Div(seratus)
	}
	return k.Nominal
}
