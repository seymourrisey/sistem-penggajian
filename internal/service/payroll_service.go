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
	GetRiwayat(ctx context.Context, karyawanID int) ([]model.PayrollRiwayat, error)
	GetLaporan(ctx context.Context, periode time.Time) ([]model.LaporanDepartemen, error)
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
// periode. Algoritma:
//
//  1. Ambil gaji_pokok karyawan.
//
//  2. Ambil seluruh komponen_gaji milik karyawan tsb.
//
//  3. Loop komponen: jika is_persen true, nilai = gaji_pokok * nominal / 100;
//     jika false, nilai = nominal apa adanya. Basis persentase SELALU
//     gaji_pokok (flat basis, bukan berjenjang) — keputusan bisnis eksplisit.
//
//  4. Sum nilai per jenis -> total_tunjangan, total_potongan.
//
//  5. gaji_bersih = gaji_pokok + total_tunjangan - total_potongan.
//
//  6. Simpan sebagai record payroll baru berstatus "draft", dibungkus dalam
//     transaksi pgx eksplisit (tx.Begin() -> Create -> tx.Commit(); jika
//     error di tengah proses, tx.Rollback())
//
//     Catatan: saat ini 'Create' adalah satu-satunya write dalam
//     transaksi ini (satu INSERT sudah atomik dengan sendirinya di
//     PostgreSQL) — pembungkusan ini demonstrasi pola untuk operasi kritis
//     yang scalable, bukan fix atas race condition yang sudah ada.
func (s *payrollService) GeneratePayroll(ctx context.Context, karyawanID int, periode time.Time) (*model.Payroll, error) {
	karyawan, err := s.karyawanRepo.GetByID(ctx, karyawanID)
	if err != nil {
		return nil, err
	}

	if karyawan.Status != model.StatusKaryawanAktif {
		return nil, repository.ErrKaryawanTidakAktif
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

	tx, err := s.payrollRepo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}

	if err := s.payrollRepo.Create(ctx, tx, payroll); err != nil {
		_ = tx.Rollback(ctx)
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		_ = tx.Rollback(ctx)
		return nil, err
	}

	return payroll, nil
}

// GetRiwayat mengambil riwayat gaji satu karyawan (F5). Memvalidasi karyawan_id
// exist lebih dulu lewat karyawanRepo.GetByID — jika tidak ditemukan,
// mengembalikan repository.ErrKaryawanNotFound (dipetakan ke 404 di handler),
// alih-alih diam-diam mengembalikan array kosong.
func (s *payrollService) GetRiwayat(ctx context.Context, karyawanID int) ([]model.PayrollRiwayat, error) {
	if _, err := s.karyawanRepo.GetByID(ctx, karyawanID); err != nil {
		return nil, err
	}
	return s.payrollRepo.GetRiwayatByKaryawanID(ctx, karyawanID)
}

// GetLaporan mengambil laporan agregat gaji per departemen untuk satu
// periode. Passthrough langsung ke repository — tidak ada validasi
// tambahan; periode yang tidak punya data payroll sama sekali akan
// menghasilkan slice kosong (bukan error), konsisten dengan sifat query
// agregat (bukan lookup by ID tunggal).
func (s *payrollService) GetLaporan(ctx context.Context, periode time.Time) ([]model.LaporanDepartemen, error) {
	return s.payrollRepo.GetLaporanAgregat(ctx, periode)
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
