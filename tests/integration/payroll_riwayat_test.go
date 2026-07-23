package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestGetRiwayatPayroll menguji GET /api/payroll/:karyawan_id/riwayat lewat
// 3 skenario: karyawan_id tidak ditemukan (404), karyawan ada tapi belum
// punya riwayat payroll (200, array kosong), dan karyawan dengan riwayat
// payroll aktual (200, data sesuai).
func TestGetRiwayatPayroll(t *testing.T) {
	t.Run("tidak_ditemukan_404", func(t *testing.T) {
		rec := doGetRiwayatRequest(t, 999999)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d, body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("kosong_200_array_kosong", func(t *testing.T) {
		// Setup: karyawan baru, belum pernah generate payroll sama sekali.
		createBody := map[string]interface{}{
			"nip":           "TPR-KOSONG-001",
			"nama":          "Dedi Kurniawan",
			"departemen_id": seedDepartemenITID,
			"jabatan":       "Staff",
			"gaji_pokok":    3500000,
			"tanggal_masuk": "2023-03-01",
		}
		createRec := doCreateKaryawanRequest(t, createBody)
		if createRec.Code != http.StatusCreated {
			t.Fatalf("setup: create karyawan harus 201, got %d, body: %s", createRec.Code, createRec.Body.String())
		}
		karyawanID := extractIDFromResponse(t, createRec)

		rec := doGetRiwayatRequest(t, karyawanID)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d, body: %s", rec.Code, rec.Body.String())
		}

		var resp []interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("gagal decode response body: %v", err)
		}
		if len(resp) != 0 {
			t.Errorf("expected array kosong, got %d item(s): %v", len(resp), resp)
		}
	})

	t.Run("sukses_200_ada_riwayat", func(t *testing.T) {
		// Setup: karyawan baru, generate payroll untuk 1 periode.
		createBody := map[string]interface{}{
			"nip":           "TPR-SUKSES-001",
			"nama":          "Eka Prasetyo",
			"departemen_id": seedDepartemenITID,
			"jabatan":       "Staff",
			"gaji_pokok":    4500000,
			"tanggal_masuk": "2023-04-01",
		}
		createRec := doCreateKaryawanRequest(t, createBody)
		if createRec.Code != http.StatusCreated {
			t.Fatalf("setup: create karyawan harus 201, got %d, body: %s", createRec.Code, createRec.Body.String())
		}
		karyawanID := extractIDFromResponse(t, createRec)

		generateBody := map[string]interface{}{
			"karyawan_id": karyawanID,
			"periode":     "2026-09-01",
		}
		generateRec := doGeneratePayrollRequest(t, generateBody)
		if generateRec.Code != http.StatusCreated {
			t.Fatalf("setup: generate payroll harus 201, got %d, body: %s", generateRec.Code, generateRec.Body.String())
		}

		rec := doGetRiwayatRequest(t, karyawanID)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d, body: %s", rec.Code, rec.Body.String())
		}

		var resp []map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("gagal decode response body: %v", err)
		}
		if len(resp) != 1 {
			t.Fatalf("expected 1 item riwayat, got %d: %v", len(resp), resp)
		}
		if resp[0]["nip"] != "TPR-SUKSES-001" {
			t.Errorf("expected nip TPR-SUKSES-001, got %v", resp[0]["nip"])
		}
		if resp[0]["periode"] != "2026-09-01" {
			t.Errorf("expected periode 2026-09-01, got %v", resp[0]["periode"])
		}
		if int(resp[0]["karyawan_id"].(float64)) != karyawanID {
			t.Errorf("expected karyawan_id %d, got %v", karyawanID, resp[0]["karyawan_id"])
		}
	})
}

// doGetRiwayatRequest adalah helper untuk mengirim
// GET /api/payroll/:karyawan_id/riwayat lewat testRouter.
func doGetRiwayatRequest(t *testing.T, karyawanID int) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/payroll/%d/riwayat", karyawanID), nil)
	rec := httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	return rec
}
