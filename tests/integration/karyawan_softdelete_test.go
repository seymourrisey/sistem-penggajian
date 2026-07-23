package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestSoftDeleteKaryawan menguji DELETE /api/karyawan/:id lewat 2 skenario:
// sukses (200, status berubah jadi "nonaktif", bukan dihapus fisik) dan id
// tidak ditemukan (404).
func TestSoftDeleteKaryawan(t *testing.T) {
	t.Run("sukses_200_status_jadi_nonaktif", func(t *testing.T) {
		// Setup: buat karyawan dulu.
		createBody := map[string]interface{}{
			"nip":           "TSD-SUKSES-001",
			"nama":          "Rina Kusuma",
			"departemen_id": seedDepartemenITID,
			"jabatan":       "Staff",
			"gaji_pokok":    3500000,
			"tanggal_masuk": "2021-08-20",
		}
		createRec := doCreateKaryawanRequest(t, createBody)
		if createRec.Code != http.StatusCreated {
			t.Fatalf("setup: create karyawan harus 201, got %d, body: %s", createRec.Code, createRec.Body.String())
		}
		id := extractIDFromResponse(t, createRec)

		// SoftDelete.
		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/karyawan/%d", id), nil)
		rec := httptest.NewRecorder()
		testRouter.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d, body: %s", rec.Code, rec.Body.String())
		}

		// Verifikasi eksplisit lewat GET: status harus "nonaktif", record
		// harus masih ada (soft-delete, bukan hard-delete — sesuai
		// ProjectDesign section 2.2).
		getReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/karyawan/%d", id), nil)
		getRec := httptest.NewRecorder()
		testRouter.ServeHTTP(getRec, getReq)

		if getRec.Code != http.StatusOK {
			t.Fatalf("expected GET setelah soft-delete tetap 200 (record masih ada), got %d, body: %s", getRec.Code, getRec.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(getRec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("gagal decode response body: %v", err)
		}
		if resp["status"] != "nonaktif" {
			t.Errorf("expected status nonaktif setelah soft-delete, got %v", resp["status"])
		}
	})

	t.Run("tidak_ditemukan_404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/karyawan/%d", notExistingID), nil)
		rec := httptest.NewRecorder()
		testRouter.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d, body: %s", rec.Code, rec.Body.String())
		}
	})
}
