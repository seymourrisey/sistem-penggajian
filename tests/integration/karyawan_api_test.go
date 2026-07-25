package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/seymourrisey/sistem-penggajian/internal/config"
	"github.com/seymourrisey/sistem-penggajian/internal/handler"
	"github.com/seymourrisey/sistem-penggajian/internal/repository"
	"github.com/seymourrisey/sistem-penggajian/internal/router"
	"github.com/seymourrisey/sistem-penggajian/internal/service"
	"github.com/seymourrisey/sistem-penggajian/pkg/database"
)

// testPool adalah connection pool yang dipakai seluruh test di package ini.
// Diinisialisasi sekali oleh TestMain, ditutup setelah seluruh test selesai.
var testPool *pgxpool.Pool

// seedDepartemenITID dan seedDepartemenHRID menyimpan id hasil INSERT seed
// departemen di TestMain. Diambil lewat RETURNING id, BUKAN diasumsikan
// bernilai tetap (1, 2, ...) — karena TRUNCATE di bawah sengaja tidak pakai
// RESTART IDENTITY (lihat komentar di TestMain), sehingga sequence terus
// naik antar run dan ID tidak predictable secara hardcode.
var (
	seedDepartemenITID int
	seedDepartemenHRID int
)

// testRouter adalah instance *gin.Engine hasil wiring penuh
// (repository → service → handler → router), dipakai seluruh test lewat
// httptest.NewRecorder() + router.ServeHTTP(rec, req). Diinisialisasi sekali
// oleh TestMain, sama untuk seluruh test case di package ini.
var testRouter *gin.Engine

// nipSeq adalah counter atomic untuk menghasilkan NIP unik antar test.
// Diperlukan karena TRUNCATE di TestMain hanya berjalan SEKALI di awal
// seluruh run (bukan per-test) — semua TestXxx dalam package ini berbagi
// state database yang sama, jadi NIP antar test WAJIB tidak pernah sama
// atau akan bentrok dengan UNIQUE constraint pada kolom karyawan.nip.
var nipSeq int64

// uniqueNIP menghasilkan NIP unik dalam satu proses test run. Sengaja
// pakai counter sekuensial (bukan timestamp) — lebih pendek (muat di
// VARCHAR(20)) dan tidak berisiko collision meski dipanggil berkali-kali
// dalam nanodetik yang sama.
func uniqueNIP() string {
	seq := atomic.AddInt64(&nipSeq, 1)
	return fmt.Sprintf("NIPTEST%d", seq)
}

// doRequest mengirim request HTTP ke testRouter secara in-memory lewat
// httptest.NewRecorder() (tanpa network nyata). body di-marshal ke JSON
// jika bukan nil; untuk request tanpa body (mis. GET/DELETE), kirim nil.
func doRequest(t *testing.T, method, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()

	var reqBody *bytes.Buffer
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("gagal marshal request body: %v", err)
		}
		reqBody = bytes.NewBuffer(b)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)
	return rec
}

// TestMain menyiapkan koneksi ke payroll_test_db sebelum seluruh test di
// package ini berjalan, dan menutup pool setelahnya. ENV_FILE di-set ke
// ".env.test" secara eksplisit agar tidak bergantung pada environment
// variable yang mungkin sudah ter-set sebelumnya di proses yang sama.
//
// Safety-check: nama database aktif diverifikasi harus persis
// "payroll_test_db" sebelum test manapun diizinkan berjalan. Ini mencegah
// integration test tanpa sadar berjalan melawan database production jika
// .env.test hilang/salah, karena LoadConfig akan fallback ke .env biasa
// tanpa error eksplisit.
func TestMain(m *testing.M) {
	os.Setenv("ENV_FILE", "../../.env.test")

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("integration test: gagal load config: %v", err)
	}

	if cfg.DBName != "payroll_test_db" {
		log.Fatalf(
			"integration test: DB_NAME aktif adalah %q, bukan payroll_test_db — "+
				"cek .env.test dan pastikan ENV_FILE terbaca dengan benar. "+
				"Test dihentikan untuk mencegah operasi tak sengaja ke database lain.",
			cfg.DBName,
		)
	}

	pool, err := database.NewPool(cfg)
	if err != nil {
		log.Fatalf("integration test: gagal konek ke %s: %v", cfg.DBName, err)
	}
	testPool = pool

	// Sanity-check tambahan: query current_database() langsung ke server,
	// bukan hanya percaya nilai cfg.DBName di sisi Go.
	var currentDB string
	if err := pool.QueryRow(context.Background(), "SELECT current_database()").Scan(&currentDB); err != nil {
		pool.Close()
		log.Fatalf("integration test: gagal verifikasi current_database(): %v", err)
	}
	if currentDB != "payroll_test_db" {
		pool.Close()
		log.Fatalf(
			"integration test: server melaporkan database aktif %q, bukan payroll_test_db. Test dihentikan.",
			currentDB,
		)
	}

	fmt.Printf("integration test: terkoneksi ke %s, mulai menjalankan test...\n", currentDB)

	// Wiring full chain: repository → service → handler → router.
	// Router butuh keempat handler meskipun scope test di package ini baru
	// karyawan — NewRouter menuntut seluruhnya sekaligus (lihat router.go).
	departemenRepo := repository.NewDepartemenRepository(testPool)
	karyawanRepo := repository.NewKaryawanRepository(testPool)
	komponenRepo := repository.NewKomponenGajiRepository(testPool)
	payrollRepo := repository.NewPayrollRepository(testPool)

	departemenSvc := service.NewDepartemenService(departemenRepo)
	karyawanSvc := service.NewKaryawanService(karyawanRepo)
	komponenSvc := service.NewKomponenGajiService(komponenRepo, karyawanRepo)
	payrollSvc := service.NewPayrollService(karyawanRepo, komponenRepo, payrollRepo)

	departemenHandler := handler.NewDepartemenHandler(departemenSvc)
	karyawanHandler := handler.NewKaryawanHandler(karyawanSvc)
	komponenHandler := handler.NewKomponenGajiHandler(komponenSvc)
	payrollHandler := handler.NewPayrollHandler(payrollSvc)

	gin.SetMode(gin.TestMode)
	testRouter = router.NewRouter(departemenHandler, karyawanHandler, komponenHandler, payrollHandler)

	// Truncate seluruh tabel yang relevan sebelum test run dimulai, supaya
	// tiap eksekusi `go test` mulai dari state kosong dan predictable dari
	// sisi DATA (bukan dari sisi nilai id). Sengaja TIDAK pakai
	// RESTART IDENTITY — itu butuh ownership atas sequence, bukan sekadar
	// privilege DML (GRANT SELECT/INSERT/UPDATE/DELETE), sehingga akan
	// memaksa payroll_test_app diberi privilege lebih tinggi dari yang
	// seharusnya (bertentangan dengan prinsip least privilege / NF5). Karena
	// itu, id HARUS selalu diambil lewat RETURNING id di tiap INSERT test —
	// jangan pernah hardcode nilai id di test manapun.
	//
	// PENTING: TRUNCATE ini hanya berjalan SEKALI di sini, bukan per-test.
	// Semua TestXxx di package ini berbagi state DB yang sama dalam satu
	// run — karena itu NIP setiap karyawan yang dibuat test manapun WAJIB
	// pakai uniqueNIP(), bukan literal string, untuk menghindari bentrok
	// UNIQUE constraint antar test.
	if _, err := testPool.Exec(context.Background(),
		"TRUNCATE departemen, karyawan, komponen_gaji, payroll CASCADE"); err != nil {
		testPool.Close()
		log.Fatalf("integration test: gagal truncate tabel sebelum test run: %v", err)
	}

	// Seed data minimal yang menjadi dependency FK wajib (karyawan.departemen_id
	// NOT NULL). Bukan data uji per skenario — hanya prasyarat struktural
	// supaya test bisa membuat karyawan sama sekali. ID hasil insert disimpan
	// ke variabel package-level lewat RETURNING id (lihat catatan di atas).
	if err := testPool.QueryRow(context.Background(),
		`INSERT INTO departemen (nama) VALUES ('IT') RETURNING id`).Scan(&seedDepartemenITID); err != nil {
		testPool.Close()
		log.Fatalf("integration test: gagal seed departemen IT: %v", err)
	}
	if err := testPool.QueryRow(context.Background(),
		`INSERT INTO departemen (nama) VALUES ('HR') RETURNING id`).Scan(&seedDepartemenHRID); err != nil {
		testPool.Close()
		log.Fatalf("integration test: gagal seed departemen HR: %v", err)
	}

	code := m.Run()

	testPool.Close()
	os.Exit(code)
}
