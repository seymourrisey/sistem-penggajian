package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestDeleteDepartemen menguji DELETE /api/departemen/:id lewat 3 skenario:
// masih direferensikan karyawan aktif (409, FK constraint), tidak
// direferensikan sama sekali (200, hard delete sukses), dan id tidak
// ditemukan (404).
//
// Departemen baru dibuat khusus di tiap subtest (bukan memakai
// seedDepartemenITID/seedDepartemenHRID package-level) supaya tidak merusak
// data yang dipakai test lain dalam test binary yang sama (strategi reset
// state truncate-sekali-di-awal, lihat TestMain).
func TestDeleteDepartemen(t *testing.T) {
	t.Run("masih_direferensikan_409", func(t *testing.T) {
		// Setup: departemen baru, khusus untuk skenario ini.
		deptRec := doCreateDepartemenRequest(t, "Departemen Uji Delete FK")
		if deptRec.Code != http.StatusCreated {
			t.Fatalf("setup: create departemen harus 201, got %d, body: %s", deptRec.Code, deptRec.Body.String())
		}
		deptID := extractIDFromResponse(t, deptRec)

		// Karyawan aktif yang mereferensikan departemen tsb.
		createBody := map[string]interface{}{
			"nip":           "TDD-FK-001",
			"nama":          "Hasan Ali",
			"departemen_id": deptID,
			"jabatan":       "Staff",
			"gaji_pokok":    3000000,
			"tanggal_masuk": "2023-07-01",
		}
		karyawanRec := doCreateKaryawanRequest(t, createBody)
		if karyawanRec.Code != http.StatusCreated {
			t.Fatalf("setup: create karyawan harus 201, got %d, body: %s", karyawanRec.Code, karyawanRec.Body.String())
		}

		// DELETE departemen yang masih direferensikan karyawan aktif.
		rec := doDeleteDepartemenRequest(t, deptID)

		if rec.Code != http.StatusConflict {
			t.Fatalf("expected status 409, got %d, body: %s", rec.Code, rec.Body.String())
		}

		// Verifikasi tambahan: departemen tetap ada (GET masih 200), bukan
		// terlanjur terhapus sebagian sebelum FK violation terjadi.
		getRec := doGetDepartemenByIDRequest(t, deptID)
		if getRec.Code != http.StatusOK {
			t.Errorf("verifikasi: departemen seharusnya masih ada (200), got %d", getRec.Code)
		}
	})

	t.Run("sukses_200_tidak_direferensikan", func(t *testing.T) {
		// Setup: departemen baru, tidak pernah dipakai karyawan manapun.
		deptRec := doCreateDepartemenRequest(t, "Departemen Uji Delete Sukses")
		if deptRec.Code != http.StatusCreated {
			t.Fatalf("setup: create departemen harus 201, got %d, body: %s", deptRec.Code, deptRec.Body.String())
		}
		deptID := extractIDFromResponse(t, deptRec)

		rec := doDeleteDepartemenRequest(t, deptID)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d, body: %s", rec.Code, rec.Body.String())
		}

		// Verifikasi tambahan: departemen benar-benar terhapus (hard delete),
		// GET setelahnya harus 404.
		getRec := doGetDepartemenByIDRequest(t, deptID)
		if getRec.Code != http.StatusNotFound {
			t.Errorf("verifikasi: departemen seharusnya sudah terhapus (404), got %d", getRec.Code)
		}
	})

	t.Run("tidak_ditemukan_404", func(t *testing.T) {
		rec := doDeleteDepartemenRequest(t, 999999)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d, body: %s", rec.Code, rec.Body.String())
		}
	})
}

// doCreateDepartemenRequest adalah helper untuk mengirim POST /api/departemen
// lewat testRouter.
func doCreateDepartemenRequest(t *testing.T, nama string) *httptest.ResponseRecorder {
	t.Helper()

	body := map[string]interface{}{"nama": nama}
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("gagal marshal request body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/departemen", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	return rec
}

// doDeleteDepartemenRequest adalah helper untuk mengirim
// DELETE /api/departemen/:id lewat testRouter.
func doDeleteDepartemenRequest(t *testing.T, id int) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/departemen/%d", id), nil)
	rec := httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	return rec
}

// doGetDepartemenByIDRequest adalah helper untuk mengirim
// GET /api/departemen/:id lewat testRouter.
func doGetDepartemenByIDRequest(t *testing.T, id int) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/departemen/%d", id), nil)
	rec := httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	return rec
}
