package unit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/seymourrisey/sistem-penggajian/internal/model"
	"github.com/seymourrisey/sistem-penggajian/internal/repository"
	"github.com/seymourrisey/sistem-penggajian/internal/service"
)

// --- Mocks ---
//
// Setiap mock meng-embed interface repository terkait (nil) lalu meng-override
// hanya method yang dipanggil payrollService. Ini menghindari asumsi terhadap
// signature method lain yang mungkin ada di interface (mis. Create/Update/Delete
// pada KaryawanRepository yang belum diverifikasi di room ini) — jika method
// yang tidak di-override sampai terpanggil, test akan panic (nil pointer),
// yang justru berguna sebagai sinyal bahwa test butuh diperbarui.

type mockKaryawanRepo struct {
	repository.KaryawanRepository
	GetByIDFunc func(ctx context.Context, id int) (*model.Karyawan, error)
}

func (m *mockKaryawanRepo) GetByID(ctx context.Context, id int) (*model.Karyawan, error) {
	return m.GetByIDFunc(ctx, id)
}

type mockKomponenGajiRepo struct {
	repository.KomponenGajiRepository
	GetByKaryawanIDFunc func(ctx context.Context, karyawanID int) ([]model.KomponenGaji, error)
}

func (m *mockKomponenGajiRepo) GetByKaryawanID(ctx context.Context, karyawanID int) ([]model.KomponenGaji, error) {
	return m.GetByKaryawanIDFunc(ctx, karyawanID)
}

// mockTx meng-embed pgx.Tx (nil) dan hanya meng-override Commit/Rollback —
// dua-duanya method yang benar-benar dipanggil payrollService.GeneratePayroll.
// Kalau service suatu saat manggil method lain dari pgx.Tx tanpa mock ini
// diupdate, test akan panic (nil pointer) — sinyal jelas kalau test perlu
// diperbarui, bukan silently pass.
type mockTx struct {
	pgx.Tx
	CommitErr error

	CommitCalls   int
	RollbackCalls int
}

func (m *mockTx) Commit(ctx context.Context) error {
	m.CommitCalls++
	return m.CommitErr
}

func (m *mockTx) Rollback(ctx context.Context) error {
	m.RollbackCalls++
	return nil
}

type mockPayrollRepo struct {
	repository.PayrollRepository
	CreateFunc func(ctx context.Context, tx pgx.Tx, p *model.Payroll) error
	BeginTxErr error

	CreateCalls    int
	CreatedPayroll *model.Payroll // gap #3: capture argumen yang benar-benar dikirim ke Create
	tx             *mockTx        // instance tx yang dikembalikan BeginTx, dipakai untuk assert Commit/Rollback
}

func (m *mockPayrollRepo) BeginTx(ctx context.Context) (pgx.Tx, error) {
	if m.BeginTxErr != nil {
		return nil, m.BeginTxErr
	}
	m.tx = &mockTx{}
	return m.tx, nil
}

func (m *mockPayrollRepo) Create(ctx context.Context, tx pgx.Tx, p *model.Payroll) error {
	m.CreateCalls++
	m.CreatedPayroll = p
	return m.CreateFunc(ctx, tx, p)
}

// --- Test ---

func TestGeneratePayroll(t *testing.T) {
	periode := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	karyawanID := 1

	errKomponenRepoGagal := errors.New("koneksi database komponen_gaji terputus")

	tunjanganFlat := model.KomponenGaji{
		KaryawanID: karyawanID, Jenis: model.JenisKomponenTunjangan,
		Nama: "Transport", Nominal: decimal.NewFromInt(500000), IsPersen: false,
	}
	tunjanganPersen := model.KomponenGaji{
		KaryawanID: karyawanID, Jenis: model.JenisKomponenTunjangan,
		Nama: "Jabatan", Nominal: decimal.NewFromInt(10), IsPersen: true, // 10% dari gaji_pokok
	}
	tunjanganPersenNol := model.KomponenGaji{
		KaryawanID: karyawanID, Jenis: model.JenisKomponenTunjangan,
		Nama: "Insentif Kondisional", Nominal: decimal.Zero, IsPersen: true, // 0% — edge case wajib
	}
	potonganFlat := model.KomponenGaji{
		KaryawanID: karyawanID, Jenis: model.JenisKomponenPotongan,
		Nama: "BPJS Kesehatan", Nominal: decimal.NewFromInt(200000), IsPersen: false,
	}
	potonganPersen := model.KomponenGaji{
		KaryawanID: karyawanID, Jenis: model.JenisKomponenPotongan,
		Nama: "BPJS Ketenagakerjaan", Nominal: decimal.NewFromInt(2), IsPersen: true, // 2%
	}

	tests := []struct {
		name string

		karyawan    *model.Karyawan
		karyawanErr error

		komponenList []model.KomponenGaji
		komponenErr  error

		createErr error

		wantErr            error
		wantTotalTunjangan decimal.Decimal
		wantTotalPotongan  decimal.Decimal
		wantGajiBersih     decimal.Decimal
		wantCreateCalled   bool

		// gap #3: kalau di-set (non-nil), verifikasi field ini di objek yang
		// benar-benar dikirim ke payrollRepo.Create, bukan cuma return value service.
		wantCreatedKaryawanID int
		checkCreatedArgs      bool

		// task #2: verifikasi transaksi pgx eksplisit benar-benar commit saat
		// sukses dan rollback saat Create gagal — bukan cuma compile.
		wantCommitCalled   bool
		wantRollbackCalled bool
	}{
		{
			name:                  "normal case: campuran flat & persen, tunjangan & potongan",
			karyawan:              &model.Karyawan{ID: karyawanID, GajiPokok: decimal.NewFromInt(5000000), Status: model.StatusKaryawanAktif},
			komponenList:          []model.KomponenGaji{tunjanganFlat, tunjanganPersen, potonganFlat, potonganPersen},
			wantTotalTunjangan:    decimal.NewFromInt(1000000), // 500000 + (10% * 5jt = 500000)
			wantTotalPotongan:     decimal.NewFromInt(300000),  // 200000 + (2% * 5jt = 100000)
			wantGajiBersih:        decimal.NewFromInt(5700000), // 5jt + 1jt - 300rb
			wantCreateCalled:      true,
			checkCreatedArgs:      true,
			wantCreatedKaryawanID: karyawanID,
			wantCommitCalled:      true,
			wantRollbackCalled:    false,
		},
		{
			name:               "gaji_pokok nol: persen ikut jadi nol, flat tetap jalan",
			karyawan:           &model.Karyawan{ID: karyawanID, GajiPokok: decimal.Zero, Status: model.StatusKaryawanAktif},
			komponenList:       []model.KomponenGaji{tunjanganPersen, potonganFlat},
			wantTotalTunjangan: decimal.Zero,                // 10% * 0 = 0
			wantTotalPotongan:  decimal.NewFromInt(200000),  // flat tidak terpengaruh gaji_pokok
			wantGajiBersih:     decimal.NewFromInt(-200000), // 0 + 0 - 200000
			wantCreateCalled:   true,
			wantCommitCalled:   true,
		},
		{
			name:               "komponen kosong: gaji_bersih = gaji_pokok apa adanya",
			karyawan:           &model.Karyawan{ID: karyawanID, GajiPokok: decimal.NewFromInt(3000000), Status: model.StatusKaryawanAktif},
			komponenList:       []model.KomponenGaji{},
			wantTotalTunjangan: decimal.Zero,
			wantTotalPotongan:  decimal.Zero,
			wantGajiBersih:     decimal.NewFromInt(3000000),
			wantCreateCalled:   true,
			wantCommitCalled:   true,
		},
		{
			name:               "is_persen true murni: hanya tunjangan persen (10%)",
			karyawan:           &model.Karyawan{ID: karyawanID, GajiPokok: decimal.NewFromInt(1000000), Status: model.StatusKaryawanAktif},
			komponenList:       []model.KomponenGaji{tunjanganPersen},
			wantTotalTunjangan: decimal.NewFromInt(100000), // 10% * 1jt
			wantTotalPotongan:  decimal.Zero,
			wantGajiBersih:     decimal.NewFromInt(1100000),
			wantCreateCalled:   true,
			wantCommitCalled:   true,
		},
		{
			// Gap #1: nominal 0% pada is_persen=true.
			name:               "is_persen true dengan nominal 0%: kontribusi harus nol, bukan galat",
			karyawan:           &model.Karyawan{ID: karyawanID, GajiPokok: decimal.NewFromInt(1000000), Status: model.StatusKaryawanAktif},
			komponenList:       []model.KomponenGaji{tunjanganPersenNol},
			wantTotalTunjangan: decimal.Zero,
			wantTotalPotongan:  decimal.Zero,
			wantGajiBersih:     decimal.NewFromInt(1000000),
			wantCreateCalled:   true,
			wantCommitCalled:   true,
		},
		{
			name:             "karyawan tidak ditemukan: harus short-circuit, Create tidak dipanggil",
			karyawanErr:      repository.ErrKaryawanNotFound,
			wantErr:          repository.ErrKaryawanNotFound,
			wantCreateCalled: false,
		},
		{
			name:             "karyawan berstatus nonaktif: harus short-circuit, Create tidak dipanggil",
			karyawan:         &model.Karyawan{ID: karyawanID, GajiPokok: decimal.NewFromInt(5000000), Status: model.StatusKaryawanNonaktif},
			wantErr:          repository.ErrKaryawanTidakAktif,
			wantCreateCalled: false,
		},
		{
			// Gap #2: repository komponen_gaji gagal.
			name:             "komponenRepo gagal: harus short-circuit, Create tidak dipanggil",
			karyawan:         &model.Karyawan{ID: karyawanID, GajiPokok: decimal.NewFromInt(5000000), Status: model.StatusKaryawanAktif},
			komponenErr:      errKomponenRepoGagal,
			wantErr:          errKomponenRepoGagal,
			wantCreateCalled: false,
		},
		{
			name:               "payroll sudah pernah digenerate untuk periode ini",
			karyawan:           &model.Karyawan{ID: karyawanID, GajiPokok: decimal.NewFromInt(4000000), Status: model.StatusKaryawanAktif},
			komponenList:       []model.KomponenGaji{},
			createErr:          repository.ErrPayrollAlreadyExists,
			wantErr:            repository.ErrPayrollAlreadyExists,
			wantCreateCalled:   true,
			wantCommitCalled:   false,
			wantRollbackCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			karyawanRepo := &mockKaryawanRepo{
				GetByIDFunc: func(ctx context.Context, id int) (*model.Karyawan, error) {
					return tt.karyawan, tt.karyawanErr
				},
			}
			komponenRepo := &mockKomponenGajiRepo{
				GetByKaryawanIDFunc: func(ctx context.Context, id int) ([]model.KomponenGaji, error) {
					return tt.komponenList, tt.komponenErr
				},
			}
			payrollRepo := &mockPayrollRepo{
				CreateFunc: func(ctx context.Context, tx pgx.Tx, p *model.Payroll) error {
					return tt.createErr
				},
			}

			svc := service.NewPayrollService(karyawanRepo, komponenRepo, payrollRepo)
			got, err := svc.GeneratePayroll(context.Background(), karyawanID, periode)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("error = %v, want %v", err, tt.wantErr)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if !got.TotalTunjangan.Equal(tt.wantTotalTunjangan) {
					t.Errorf("TotalTunjangan = %s, want %s", got.TotalTunjangan, tt.wantTotalTunjangan)
				}
				if !got.TotalPotongan.Equal(tt.wantTotalPotongan) {
					t.Errorf("TotalPotongan = %s, want %s", got.TotalPotongan, tt.wantTotalPotongan)
				}
				if !got.GajiBersih.Equal(tt.wantGajiBersih) {
					t.Errorf("GajiBersih = %s, want %s", got.GajiBersih, tt.wantGajiBersih)
				}
				if got.Status != model.StatusPayrollDraft {
					t.Errorf("Status = %s, want %s", got.Status, model.StatusPayrollDraft)
				}
			}

			createCalled := payrollRepo.CreateCalls > 0
			if createCalled != tt.wantCreateCalled {
				t.Errorf("Create called = %v, want %v", createCalled, tt.wantCreateCalled)
			}

			// Gap #3: verifikasi argumen yang benar-benar dikirim ke Create.
			if tt.checkCreatedArgs {
				if payrollRepo.CreatedPayroll == nil {
					t.Fatalf("checkCreatedArgs=true tapi payrollRepo.CreatedPayroll nil (Create tidak pernah dipanggil?)")
				}
				if payrollRepo.CreatedPayroll.KaryawanID != tt.wantCreatedKaryawanID {
					t.Errorf("Create dipanggil dengan KaryawanID = %d, want %d",
						payrollRepo.CreatedPayroll.KaryawanID, tt.wantCreatedKaryawanID)
				}
				if !payrollRepo.CreatedPayroll.Periode.Equal(periode) {
					t.Errorf("Create dipanggil dengan Periode = %v, want %v",
						payrollRepo.CreatedPayroll.Periode, periode)
				}
			}

			// Task #2: verifikasi transaksi pgx benar-benar di-commit saat sukses,
			// di-rollback saat Create gagal. Kalau BeginTx tidak pernah dipanggil
			// (mis. karyawan/komponen error, short-circuit sebelum tx dibuka),
			// payrollRepo.tx tetap nil — wajar, bukan bug.
			if payrollRepo.tx != nil {
				if payrollRepo.tx.CommitCalls != boolToInt(tt.wantCommitCalled) {
					t.Errorf("Commit calls = %d, want %d", payrollRepo.tx.CommitCalls, boolToInt(tt.wantCommitCalled))
				}
				if payrollRepo.tx.RollbackCalls != boolToInt(tt.wantRollbackCalled) {
					t.Errorf("Rollback calls = %d, want %d", payrollRepo.tx.RollbackCalls, boolToInt(tt.wantRollbackCalled))
				}
			} else if tt.wantCommitCalled || tt.wantRollbackCalled {
				t.Errorf("tx tidak pernah dibuka (BeginTx tidak dipanggil), padahal wantCommitCalled=%v wantRollbackCalled=%v",
					tt.wantCommitCalled, tt.wantRollbackCalled)
			}
		})
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
