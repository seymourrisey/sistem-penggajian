package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestCreateKomponenGaji menguji endpoint
// POST /api/karyawan/:id/komponen-gaji.
//
// Fokus integration test ini adalah memverifikasi kontrak HTTP,
// validasi business rule, serta memastikan perubahan benar-benar
// tersimpan (atau tidak tersimpan) di database.
func TestCreateKomponenGaji(t *testing.T) {
	t.Run("sukses_201", func(t *testing.T) {
		createBody := map[string]interface{}{
			"nip":           "TKGCREATE-001",
			"nama":          "Andi Saputra",
			"departemen_id": seedDepartemenITID,
			"jabatan":       "Backend Developer",
			"gaji_pokok":    6000000,
			"tanggal_masuk": "2024-01-01",
		}

		createRec := doCreateKaryawanRequest(t, createBody)
		if createRec.Code != http.StatusCreated {
			t.Fatalf("setup: create karyawan harus 201, got %d, body: %s",
				createRec.Code, createRec.Body.String())
		}

		karyawanID := extractIDFromResponse(t, createRec)

		body := map[string]interface{}{
			"jenis":     "tunjangan",
			"nama":      "Tunjangan Transport",
			"nominal":   500000,
			"is_persen": false,
		}

		rec := doCreateKomponenGajiRequest(t, karyawanID, body)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d, body: %s",
				rec.Code, rec.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("gagal decode response: %v", err)
		}

		if resp["nama"] != "Tunjangan Transport" {
			t.Errorf("expected nama 'Tunjangan Transport', got %v", resp["nama"])
		}

		if resp["jenis"] != "tunjangan" {
			t.Errorf("expected jenis 'tunjangan', got %v", resp["jenis"])
		}

		if resp["is_persen"] != false {
			t.Errorf("expected is_persen=false, got %v", resp["is_persen"])
		}

		// Verifikasi data benar-benar tersimpan di database.
		var count int
		err := testPool.QueryRow(
			context.Background(),
			`SELECT COUNT(*)
			 FROM komponen_gaji
			 WHERE karyawan_id=$1
			   AND nama=$2`,
			karyawanID,
			"Tunjangan Transport",
		).Scan(&count)
		if err != nil {
			t.Fatalf("gagal query database: %v", err)
		}

		if count != 1 {
			t.Errorf("expected 1 row inserted, got %d", count)
		}
	})

	// Test karyawan nonaktif (harus gagal).
	t.Run("karyawan_nonaktif_400", func(t *testing.T) {
		createBody := map[string]interface{}{
			"nip":           "TKGCREATE-002",
			"nama":          "Budi Hartono",
			"departemen_id": seedDepartemenITID,
			"jabatan":       "Programmer",
			"gaji_pokok":    5000000,
			"tanggal_masuk": "2023-01-01",
		}

		createRec := doCreateKaryawanRequest(t, createBody)
		if createRec.Code != http.StatusCreated {
			t.Fatalf("setup create karyawan gagal: %s", createRec.Body.String())
		}

		karyawanID := extractIDFromResponse(t, createRec)

		deleteReq := httptest.NewRequest(
			http.MethodDelete,
			fmt.Sprintf("/api/karyawan/%d", karyawanID),
			nil,
		)

		deleteRec := httptest.NewRecorder()
		testRouter.ServeHTTP(deleteRec, deleteReq)

		if deleteRec.Code != http.StatusOK {
			t.Fatalf("setup soft delete gagal: %s", deleteRec.Body.String())
		}

		body := map[string]interface{}{
			"jenis":     "tunjangan",
			"nama":      "Tunjangan Transport",
			"nominal":   500000,
			"is_persen": false,
		}

		rec := doCreateKomponenGajiRequest(t, karyawanID, body)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d, body: %s",
				rec.Code, rec.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("gagal decode response: %v", err)
		}

		if resp["error"] != "karyawan tidak aktif" {
			t.Errorf("expected error 'karyawan tidak aktif', got %v", resp["error"])
		}

		// Verifikasi request ditolak sebelum INSERT.
		var count int
		err := testPool.QueryRow(
			context.Background(),
			`SELECT COUNT(*)
			 FROM komponen_gaji
			 WHERE karyawan_id=$1`,
			karyawanID,
		).Scan(&count)

		if err != nil {
			t.Fatalf("gagal query database: %v", err)
		}

		if count != 0 {
			t.Errorf("expected 0 row inserted, got %d", count)
		}
	})

	// Test: nama komponen gaji kosong, harus 400.
	t.Run("nama_kosong_400", func(t *testing.T) {
		// Setup: karyawan aktif.
		createBody := map[string]interface{}{
			"nip":           "TKGCREATE-003",
			"nama":          "Candra Wijaya",
			"departemen_id": seedDepartemenITID,
			"jabatan":       "Staff",
			"gaji_pokok":    4500000,
			"tanggal_masuk": "2024-02-01",
		}

		createRec := doCreateKaryawanRequest(t, createBody)
		if createRec.Code != http.StatusCreated {
			t.Fatalf("setup: create karyawan harus 201, got %d, body: %s",
				createRec.Code, createRec.Body.String())
		}

		karyawanID := extractIDFromResponse(t, createRec)

		body := map[string]interface{}{
			"jenis":     "tunjangan",
			"nama":      "",
			"nominal":   250000,
			"is_persen": false,
		}

		rec := doCreateKomponenGajiRequest(t, karyawanID, body)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d, body: %s",
				rec.Code, rec.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("gagal decode response: %v", err)
		}

		if resp["error"] != "input tidak lengkap: pastikan semua field wajib sudah diisi" {
			t.Errorf("unexpected error message: %v", resp["error"])
		}

		var count int
		err := testPool.QueryRow(
			context.Background(),
			`SELECT COUNT(*) FROM komponen_gaji WHERE karyawan_id=$1`,
			karyawanID,
		).Scan(&count)
		if err != nil {
			t.Fatalf("gagal query database: %v", err)
		}

		if count != 0 {
			t.Errorf("expected 0 row inserted, got %d", count)
		}
	})

	t.Run("jenis_tidak_valid_400", func(t *testing.T) {
		// Setup: karyawan aktif.
		createBody := map[string]interface{}{
			"nip":           "TKGCREATE-004",
			"nama":          "Dewi Anggraini",
			"departemen_id": seedDepartemenITID,
			"jabatan":       "Staff",
			"gaji_pokok":    4500000,
			"tanggal_masuk": "2024-03-01",
		}

		createRec := doCreateKaryawanRequest(t, createBody)
		if createRec.Code != http.StatusCreated {
			t.Fatalf("setup: create karyawan harus 201, got %d, body: %s",
				createRec.Code, createRec.Body.String())
		}

		karyawanID := extractIDFromResponse(t, createRec)

		body := map[string]interface{}{
			"jenis":     "bonus",
			"nama":      "Bonus Tahunan",
			"nominal":   1000000,
			"is_persen": false,
		}

		rec := doCreateKomponenGajiRequest(t, karyawanID, body)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d, body: %s",
				rec.Code, rec.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("gagal decode response: %v", err)
		}

		if resp["error"] != "jenis komponen gaji harus 'tunjangan' atau 'potongan'" {
			t.Errorf("unexpected error message: %v", resp["error"])
		}

		var count int
		err := testPool.QueryRow(
			context.Background(),
			`SELECT COUNT(*) FROM komponen_gaji WHERE karyawan_id=$1`,
			karyawanID,
		).Scan(&count)
		if err != nil {
			t.Fatalf("gagal query database: %v", err)
		}

		if count != 0 {
			t.Errorf("expected 0 row inserted, got %d", count)
		}
	})

}
