package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/seymourrisey/sistem-penggajian/internal/model"
)

var ErrPayrollNotFound = errors.New("payroll not found")

var ErrPayrollAlreadyExists = errors.New("payroll already exists for this karyawan and periode")

// PayrollRepository mendefinisikan operasi akses data untuk tabel payroll,
// termasuk query gabungan untuk riwayat dan laporan agregat (F5, F6).
type PayrollRepository interface {
	Create(ctx context.Context, p *model.Payroll) error
	GetRiwayatByKaryawanID(ctx context.Context, karyawanID int) ([]model.PayrollRiwayat, error)
	GetLaporanAgregat(ctx context.Context, periode time.Time) ([]model.LaporanDepartemen, error)
}

type payrollRepository struct {
	db *pgxpool.Pool
}

func NewPayrollRepository(db *pgxpool.Pool) PayrollRepository {
	return &payrollRepository{db: db}
}

// Create menyisipkan record payroll baru dan mengisi field ID & CreatedAt
// hasil generate PostgreSQL ke struct p. Jika kombinasi karyawan_id+periode
// sudah ada (pelanggaran constraint unik), mengembalikan
// ErrPayrollAlreadyExists alih-alih error pgx mentah.
func (r *payrollRepository) Create(ctx context.Context, p *model.Payroll) error {
	query := `
		INSERT INTO payroll (karyawan_id, periode, gaji_pokok, total_tunjangan, total_potongan, gaji_bersih, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at`

	err := r.db.QueryRow(ctx, query,
		p.KaryawanID, p.Periode, p.GajiPokok, p.TotalTunjangan, p.TotalPotongan, p.GajiBersih, p.Status,
	).Scan(&p.ID, &p.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrPayrollAlreadyExists
		}
		return err
	}
	return nil
}

// GetRiwayatByKaryawanID mengambil riwayat gaji karyawan (F5). Query JOIN
// payroll ke karyawan agar nip & nama ikut tersedia tanpa query terpisah.
// Mengembalikan slice kosong (bukan error) jika karyawan belum punya slip.
func (r *payrollRepository) GetRiwayatByKaryawanID(ctx context.Context, karyawanID int) ([]model.PayrollRiwayat, error) {
	query := `
		SELECT
			p.id, p.karyawan_id, k.nip, k.nama,
			p.periode, p.gaji_pokok, p.total_tunjangan, p.total_potongan, p.gaji_bersih,
			p.status, p.created_at
		FROM payroll p
		JOIN karyawan k ON k.id = p.karyawan_id
		WHERE p.karyawan_id = $1
		ORDER BY p.periode DESC`

	rows, err := r.db.Query(ctx, query, karyawanID)
	if err != nil {
		return nil, err
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
			return nil, err
		}
		result = append(result, pr)
	}
	if err := rows.Err(); err != nil {
		return nil, err
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
			d.id,
			d.nama,
			COUNT(p.id)            AS jumlah_karyawan,
			SUM(p.gaji_bersih)     AS total_gaji_bersih,
			AVG(p.gaji_bersih)     AS rata_rata_gaji_bersih
		FROM payroll p
		JOIN karyawan k    ON k.id = p.karyawan_id
		JOIN departemen d  ON d.id = k.departemen_id
		WHERE p.periode = $1
		GROUP BY d.id, d.nama
		ORDER BY d.nama`

	rows, err := r.db.Query(ctx, query, periode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []model.LaporanDepartemen
	for rows.Next() {
		var l model.LaporanDepartemen
		if err := rows.Scan(
			&l.DepartemenID, &l.NamaDepartemen,
			&l.JumlahKaryawan, &l.TotalGajiBersih, &l.RataRataGajiBersih,
		); err != nil {
			return nil, err
		}
		l.Periode = periode
		result = append(result, l)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
