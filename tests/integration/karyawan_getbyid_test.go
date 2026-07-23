package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestGetKaryawanByID menguji GET /api/karyawan/:id lewat 3 skenario:
// sukses (200), id tidak ditemukan di database (404), dan id dengan format
// bukan angka (400, gagal di strconv.Atoi sebelum sempat query service).
func TestGetKaryawanByID(t *testing.T) {
	t.Run("sukses_200", func(t *testing.T) {
		// Setup: buat karyawan dulu lewat endpoint Create, ambil id dari response.
		createBody := map[string]interface{}{
			"nip":           "TGK-SUKSES-001",
			"nama":          "Siti Aminah",
			"departemen_id": seedDepartemenITID,
			"jabatan":       "Staff",
			"gaji_pokok":    4500000,
			"tanggal_masuk": "2023-06-01",
		}
		createRec := doCreateKaryawanRequest(t, createBody)
		if createRec.Code != http.StatusCreated {
			t.Fatalf("setup: create karyawan harus 201, got %d, body: %s", createRec.Code, createRec.Body.String())
		}
		var created map[string]interface{}
		if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
			t.Fatalf("setup: gagal decode response create: %v", err)
		}
		idFloat, ok := created["id"].(float64) // JSON number selalu ke-decode sebagai float64
		if !ok {
			t.Fatalf("setup: field id tidak ada atau bukan angka di response create: %v", created)
		}
		id := int(idFloat)

		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/karyawan/%d", id), nil)
		rec := httptest.NewRecorder()
		testRouter.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d, body: %s", rec.Code, rec.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("gagal decode response body: %v", err)
		}
		if resp["nip"] != "TGK-SUKSES-001" {
			t.Errorf("expected nip TGK-SUKSES-001, got %v", resp["nip"])
		}
	})

	t.Run("tidak_ditemukan_404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/karyawan/%d", notExistingID), nil)
		rec := httptest.NewRecorder()
		testRouter.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d, body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("id_format_invalid_400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/karyawan/bukan-angka", nil)
		rec := httptest.NewRecorder()
		testRouter.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d, body: %s", rec.Code, rec.Body.String())
		}
	})
}
