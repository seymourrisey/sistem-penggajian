package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/seymourrisey/sistem-penggajian/internal/model"
)

// ErrPayrollAlreadyExists dikembalikan ketika payroll untuk kombinasi
// karyawan_id dan periode yang sama sudah pernah digenerate (mapping dari
// pelanggaran UNIQUE constraint payroll(karyawan_id, periode)).
var ErrPayrollAlreadyExists = errors.New("payroll sudah ada untuk karyawan dan periode ini")

// ErrKaryawanTidakAktif dikembalikan ketika karyawan tidak aktif.
var ErrKaryawanTidakAktif = errors.New("karyawan tidak aktif")

// PayrollRepository mendefinisikan operasi akses data untuk tabel payroll,
// termasuk query gabungan untuk riwayat dan laporan agregat (F5, F6).
type PayrollRepository interface {
	// BeginTx membuka transaksi pgx baru. Caller (service layer)
	// bertanggung jawab memanggil tx.Commit(ctx) atau tx.Rollback(ctx).
	BeginTx(ctx context.Context) (pgx.Tx, error)

	// Create menyisipkan record payroll baru di dalam transaksi tx yang
	// sudah dibuka lewat BeginTx. Tidak melakukan commit/rollback sendiri —
	// itu tanggung jawab caller, supaya bisa dibungkus bersama operasi
	// kritis lain di masa depan tanpa mengubah signature ini lagi.
	Create(ctx context.Context, tx pgx.Tx, p *model.Payroll) error

	GetRiwayatByKaryawanID(ctx context.Context, karyawanID int) ([]model.PayrollRiwayat, error)
	GetLaporanAgregat(ctx context.Context, periode time.Time) ([]model.LaporanDepartemen, error)
}

// payrollRepository adalah implementasi PayrollRepository menggunakan pgx pool.
type payrollRepository struct {
	db *pgxpool.Pool
}

// NewPayrollRepository membuat instance baru PayrollRepository.
func NewPayrollRepository(db *pgxpool.Pool) PayrollRepository {
	return &payrollRepository{db: db}
}

// BeginTx membuka transaksi pgx baru dari pool. Dipanggil dari service layer
// sebelum operasi write kritis (mis. GeneratePayroll) — bukti KUK unit #2
// "transaksi eksplisit (COMMIT/ROLLBACK) untuk operasi kritis" (NF5).
func (r *payrollRepository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.db.Begin(ctx)
}

// Create menyisipkan record payroll baru di dalam transaksi tx dan mengisi
// field ID & CreatedAt hasil generate PostgreSQL ke struct p. Jika kombinasi
// karyawan_id+periode sudah ada (pelanggaran constraint unik), mengembalikan
// ErrPayrollAlreadyExists alih-alih error pgx mentah.
//
// CATATAN: dieksekusi lewat tx (bukan r.db) — caller wajib Commit/Rollback
// tx tersebut sendiri, function ini tidak melakukannya.
//
// Repository hanya melakukan persistence payroll.
// Seluruh perhitungan payroll telah selesai di service.
// Repository tidak menggunakan stored procedure SQL Native;
// procedure sp_generate_payroll_snapshot hanya disediakan
// sebagai artefak demonstrasi SQL Native dan tidak menjadi
// jalur eksekusi aplikasi.
func (r *payrollRepository) Create(ctx context.Context, tx pgx.Tx, p *model.Payroll) error {
	query := `
		INSERT INTO payroll (karyawan_id, periode, gaji_pokok, total_tunjangan, total_potongan, gaji_bersih, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING payroll_id, created_at`

	err := tx.QueryRow(ctx, query,
		p.KaryawanID, p.Periode, p.GajiPokok, p.TotalTunjangan, p.TotalPotongan, p.GajiBersih, p.Status,
	).Scan(&p.ID, &p.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrPayrollAlreadyExists
		}
		return fmt.Errorf("gagal insert payroll: %w", err)
	}
	return nil
}

// GetRiwayatByKaryawanID mengambil riwayat gaji karyawan (F5). Query JOIN
// payroll ke karyawan agar nip & nama ikut tersedia tanpa query terpisah.
// Mengembalikan slice kosong (bukan error) jika karyawan belum punya slip.
func (r *payrollRepository) GetRiwayatByKaryawanID(ctx context.Context, karyawanID int) ([]model.PayrollRiwayat, error) {
	query := `
		SELECT
			p.payroll_id, p.karyawan_id, k.nip, k.nama_karyawan,
			p.periode, p.gaji_pokok, p.total_tunjangan, p.total_potongan, p.gaji_bersih,
			p.status, p.created_at
		FROM payroll p
		JOIN karyawan k ON k.karyawan_id = p.karyawan_id
		WHERE p.karyawan_id = $1
		ORDER BY p.periode DESC`

	rows, err := r.db.Query(ctx, query, karyawanID)
	if err != nil {
		return nil, fmt.Errorf("gagal ambil riwayat payroll karyawan_id=%d: %w", karyawanID, err)
	}
	defer rows.Close()

	var result []model.PayrollRiwayat
	for rows.Next() {
		var pr model.PayrollRiwayat
		if err := rows.Scan(
			&pr.ID, &pr.KaryawanID, &pr.NIP, &pr.NamaKaryawan,
			&pr.Periode, &pr.GajiPokok, &pr.TotalTunjangan, &pr.TotalPotongan, &pr.GajiBersih,
			&pr.Status, &pr.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("gagal scan riwayat payroll karyawan_id=%d: %w", karyawanID, err)
		}
		result = append(result, pr)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterasi riwayat payroll karyawan_id=%d: %w", karyawanID, err)
	}
	return result, nil
}

// GetLaporanAgregat mengambil laporan agregat gaji per departemen untuk satu
// periode (F6) — bukti utama unit kompetensi #2 (SQL): JOIN 3 tabel
// (payroll, karyawan, departemen) + GROUP BY + COUNT/SUM/AVG.
//
// Hanya departemen yang memiliki minimal satu slip payroll pada periode
// tsb yang muncul di hasil (inner JOIN, bukan LEFT JOIN) — sesuai
// kebutuhan laporan "gaji per departemen per periode" pada F6.
func (r *payrollRepository) GetLaporanAgregat(ctx context.Context, periode time.Time) ([]model.LaporanDepartemen, error) {
	query := `
		SELECT
			d.departemen_id,
			d.nama_departemen,
			COUNT(p.payroll_id)            AS jumlah_karyawan,
			SUM(p.gaji_bersih)     AS total_gaji_bersih,
			AVG(p.gaji_bersih)     AS rata_rata_gaji_bersih
		FROM payroll p
		JOIN karyawan k    ON k.karyawan_id = p.karyawan_id
		JOIN departemen d  ON d.departemen_id = k.departemen_id
		WHERE p.periode = $1
		GROUP BY d.departemen_id, d.nama_departemen
		ORDER BY d.nama_departemen`

	rows, err := r.db.Query(ctx, query, periode)
	if err != nil {
		return nil, fmt.Errorf("gagal ambil laporan payroll periode=%s: %w", periode.Format(time.DateOnly), err)
	}
	defer rows.Close()

	var result []model.LaporanDepartemen
	for rows.Next() {
		var l model.LaporanDepartemen
		if err := rows.Scan(
			&l.DepartemenID, &l.NamaDepartemen,
			&l.JumlahKaryawan, &l.TotalGajiBersih, &l.RataRataGajiBersih,
		); err != nil {
			return nil, fmt.Errorf("gagal scan laporan departemen periode=%s: %w", periode.Format(time.DateOnly), err)
		}
		l.Periode = periode
		result = append(result, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterasi laporan departemen periode=%s: %w", periode.Format(time.DateOnly), err)
	}
	return result, nil
}
