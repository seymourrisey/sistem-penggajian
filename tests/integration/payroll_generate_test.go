package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shopspring/decimal"
)

// TestGeneratePayroll menguji POST /api/payroll/generate lewat 2 skenario:
// kalkulasi gaji_bersih end-to-end yang benar (201), dan penolakan duplikat
// karyawan_id+periode yang sama (409, UNIQUE constraint).
func TestGeneratePayroll(t *testing.T) {
	t.Run("sukses_201_kalkulasi_benar", func(t *testing.T) {
		// Setup: karyawan dengan gaji_pokok 5.000.000.
		createBody := map[string]interface{}{
			"nip":           "TPG-SUKSES-001",
			"nama":          "Budi Santoso",
			"departemen_id": seedDepartemenITID,
			"jabatan":       "Staff",
			"gaji_pokok":    5000000,
			"tanggal_masuk": "2023-01-10",
		}
		createRec := doCreateKaryawanRequest(t, createBody)
		if createRec.Code != http.StatusCreated {
			t.Fatalf("setup: create karyawan harus 201, got %d, body: %s", createRec.Code, createRec.Body.String())
		}
		karyawanID := extractIDFromResponse(t, createRec)

		// Tunjangan tetap: 500.000 (nominal apa adanya, is_persen=false).
		tunjanganBody := map[string]interface{}{
			"jenis":     "tunjangan",
			"nama":      "Tunjangan Transport",
			"nominal":   500000,
			"is_persen": false,
		}
		tunjanganRec := doCreateKomponenGajiRequest(t, karyawanID, tunjanganBody)
		if tunjanganRec.Code != http.StatusCreated {
			t.Fatalf("setup: create tunjangan harus 201, got %d, body: %s", tunjanganRec.Code, tunjanganRec.Body.String())
		}

		// Potongan persen: 2% dari gaji_pokok -> 5.000.000 * 2/100 = 100.000.
		// Basis persentase selalu gaji_pokok (flat basis), sesuai
		// payroll_service.go GeneratePayroll.
		potonganBody := map[string]interface{}{
			"jenis":     "potongan",
			"nama":      "BPJS Kesehatan",
			"nominal":   2,
			"is_persen": true,
		}
		potonganRec := doCreateKomponenGajiRequest(t, karyawanID, potonganBody)
		if potonganRec.Code != http.StatusCreated {
			t.Fatalf("setup: create potongan harus 201, got %d, body: %s", potonganRec.Code, potonganRec.Body.String())
		}

		// Expected: gaji_bersih = 5.000.000 + 500.000 - 100.000 = 5.400.000
		expectedTunjangan := decimal.NewFromInt(500000)
		expectedPotongan := decimal.NewFromInt(100000)
		expectedGajiBersih := decimal.NewFromInt(5400000)

		generateBody := map[string]interface{}{
			"karyawan_id": karyawanID,
			"periode":     "2026-08-01",
		}
		rec := doGeneratePayrollRequest(t, generateBody)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d, body: %s", rec.Code, rec.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("gagal decode response body: %v", err)
		}

		actualTunjangan := parseDecimalField(t, resp, "total_tunjangan")
		actualPotongan := parseDecimalField(t, resp, "total_potongan")
		actualGajiBersih := parseDecimalField(t, resp, "gaji_bersih")

		if !actualTunjangan.Equal(expectedTunjangan) {
			t.Errorf("expected total_tunjangan %s, got %s", expectedTunjangan, actualTunjangan)
		}
		if !actualPotongan.Equal(expectedPotongan) {
			t.Errorf("expected total_potongan %s, got %s", expectedPotongan, actualPotongan)
		}
		if !actualGajiBersih.Equal(expectedGajiBersih) {
			t.Errorf("expected gaji_bersih %s, got %s", expectedGajiBersih, actualGajiBersih)
		}
		if resp["status"] != "draft" {
			t.Errorf("expected status draft, got %v", resp["status"])
		}
	})

	t.Run("duplikat_periode_409", func(t *testing.T) {
		// Setup: karyawan baru, tanpa komponen gaji (tidak relevan untuk
		// skenario ini, fokus hanya pada UNIQUE constraint karyawan_id+periode).
		createBody := map[string]interface{}{
			"nip":           "TPG-DUPLIKAT-001",
			"nama":          "Citra Dewi",
			"departemen_id": seedDepartemenITID,
			"jabatan":       "Staff",
			"gaji_pokok":    4000000,
			"tanggal_masuk": "2023-02-01",
		}
		createRec := doCreateKaryawanRequest(t, createBody)
		if createRec.Code != http.StatusCreated {
			t.Fatalf("setup: create karyawan harus 201, got %d, body: %s", createRec.Code, createRec.Body.String())
		}
		karyawanID := extractIDFromResponse(t, createRec)

		generateBody := map[string]interface{}{
			"karyawan_id": karyawanID,
			"periode":     "2026-08-01",
		}

		// Generate pertama: harus sukses.
		firstRec := doGeneratePayrollRequest(t, generateBody)
		if firstRec.Code != http.StatusCreated {
			t.Fatalf("setup: generate payroll pertama harus 201, got %d, body: %s", firstRec.Code, firstRec.Body.String())
		}

		// Generate kedua, kombinasi karyawan_id+periode sama persis: harus ditolak.
		secondRec := doGeneratePayrollRequest(t, generateBody)
		if secondRec.Code != http.StatusConflict {
			t.Fatalf("expected status 409, got %d, body: %s", secondRec.Code, secondRec.Body.String())
		}

		// Verifikasi: tidak ada duplikasi data payroll.
		var count int
		err := testPool.QueryRow(context.Background(),
			`SELECT COUNT(*) FROM payroll WHERE karyawan_id = $1 AND periode = $2`,
			karyawanID, "2026-08-01").Scan(&count)
		if err != nil {
			t.Fatalf("gagal query count payroll: %v", err)
		}
		if count != 1 {
			t.Errorf("expected exactly 1 payroll row (no partial/duplicate commit), got %d", count)
		}
	})

	t.Run("karyawan_nonaktif_400", func(t *testing.T) {
		// Setup: karyawan baru, lalu soft-delete (jadi nonaktif) sebelum generate.
		createBody := map[string]interface{}{
			"nip":           "TPG-NONAKTIF-001",
			"nama":          "Dewi Lestari",
			"departemen_id": seedDepartemenITID,
			"jabatan":       "Staff",
			"gaji_pokok":    4500000,
			"tanggal_masuk": "2023-03-01",
		}
		createRec := doCreateKaryawanRequest(t, createBody)
		if createRec.Code != http.StatusCreated {
			t.Fatalf("setup: create karyawan harus 201, got %d, body: %s", createRec.Code, createRec.Body.String())
		}
		karyawanID := extractIDFromResponse(t, createRec)

		// Soft-delete langsung lewat DELETE /api/karyawan/:id (inline, tanpa
		// helper terpisah, supaya file ini tidak bergantung pada helper yang
		// didefinisikan di karyawan_softdelete_test.go).
		deleteReq := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/karyawan/%d", karyawanID), nil)
		deleteRec := httptest.NewRecorder()
		testRouter.ServeHTTP(deleteRec, deleteReq)
		if deleteRec.Code != http.StatusOK {
			t.Fatalf("setup: soft-delete karyawan harus 200, got %d, body: %s", deleteRec.Code, deleteRec.Body.String())
		}

		generateBody := map[string]interface{}{
			"karyawan_id": karyawanID,
			"periode":     "2026-08-01",
		}
		rec := doGeneratePayrollRequest(t, generateBody)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d, body: %s", rec.Code, rec.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("gagal decode response body: %v", err)
		}
		if resp["error"] != "karyawan tidak aktif" {
			t.Errorf("expected error message 'karyawan tidak aktif', got %v", resp["error"])
		}

		// Verifikasi: tidak ada row payroll yang ke-insert untuk karyawan ini.
		var count int
		err := testPool.QueryRow(context.Background(),
			`SELECT COUNT(*) FROM payroll WHERE karyawan_id = $1 AND periode = $2`,
			karyawanID, "2026-08-01").Scan(&count)
		if err != nil {
			t.Fatalf("gagal query count payroll: %v", err)
		}
		if count != 0 {
			t.Errorf("expected 0 payroll row (request ditolak sebelum insert), got %d", count)
		}
	})

	// Test: format_periode_salah_400
	t.Run("format_periode_salah_400", func(t *testing.T) {
		// Setup: buat karyawan aktif.
		createBody := map[string]interface{}{
			"nip":           "TPG-FORMAT-001",
			"nama":          "Budi Santoso",
			"departemen_id": seedDepartemenITID,
			"jabatan":       "Staff",
			"gaji_pokok":    5000000,
			"tanggal_masuk": "2023-01-10",
		}

		createRec := doCreateKaryawanRequest(t, createBody)
		if createRec.Code != http.StatusCreated {
			t.Fatalf("setup: create karyawan harus 201, got %d, body: %s",
				createRec.Code,
				createRec.Body.String())
		}

		karyawanID := extractIDFromResponse(t, createRec)

		// Verifikasi jumlah payroll sebelum request.
		var before int
		err := testPool.QueryRow(
			context.Background(),
			`SELECT COUNT(*) FROM payroll`,
		).Scan(&before)

		if err != nil {
			t.Fatalf("gagal query database: %v", err)
		}

		generateBody := map[string]interface{}{
			"karyawan_id": karyawanID,
			"periode":     "<a.981", // format yang salah
		}

		rec := doGeneratePayrollRequest(t, generateBody)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d, body: %s",
				rec.Code,
				rec.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("gagal decode response: %v", err)
		}

		if resp["error"] != "periode harus format YYYY-MM-DD" {
			t.Errorf("expected error 'periode harus format YYYY-MM-DD', got %v",
				resp["error"])
		}

		// Verifikasi tidak ada payroll baru.
		var after int
		err = testPool.QueryRow(
			context.Background(),
			`SELECT COUNT(*) FROM payroll`,
		).Scan(&after)

		if err != nil {
			t.Fatalf("gagal query database: %v", err)
		}

		if after != before {
			t.Errorf("expected no new payroll row, before=%d after=%d",
				before, after)
		}
	})
}

// doCreateKomponenGajiRequest adalah helper untuk mengirim
// POST /api/karyawan/:id/komponen-gaji lewat testRouter.
func doCreateKomponenGajiRequest(t *testing.T, karyawanID int, body map[string]interface{}) *httptest.ResponseRecorder {
	t.Helper()

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("gagal marshal request body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/karyawan/%d/komponen-gaji", karyawanID), bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	return rec
}

// doGeneratePayrollRequest adalah helper untuk mengirim POST /api/payroll/generate
// lewat testRouter.
func doGeneratePayrollRequest(t *testing.T, body map[string]interface{}) *httptest.ResponseRecorder {
	t.Helper()

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("gagal marshal request body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/payroll/generate", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	return rec
}

// parseDecimalField mengambil field bertipe decimal.Decimal dari response JSON
// (di-marshal sebagai string oleh shopspring/decimal) dan mem-parse-nya
// menjadi decimal.Decimal untuk perbandingan numerik yang presisi (bukan
// perbandingan string, karena representasi trailing zero bisa berbeda,
// mis. "500000" vs "500000.00").
func parseDecimalField(t *testing.T, resp map[string]interface{}, field string) decimal.Decimal {
	t.Helper()

	raw, ok := resp[field].(string)
	if !ok {
		t.Fatalf("field %s tidak ada atau bukan string di response: %v", field, resp)
	}
	val, err := decimal.NewFromString(raw)
	if err != nil {
		t.Fatalf("gagal parse field %s (%q) sebagai decimal: %v", field, raw, err)
	}
	return val
}
