package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestUpdateKaryawan menguji PUT /api/karyawan/:id lewat 3 skenario:
// sukses (200), id tidak ditemukan (404), dan validasi input gagal (400).
func TestUpdateKaryawan(t *testing.T) {
	t.Run("sukses_200", func(t *testing.T) {
		// Setup: buat karyawan dulu.
		createBody := map[string]interface{}{
			"nip":           "TUK-SUKSES-001",
			"nama":          "Andi Wijaya",
			"departemen_id": seedDepartemenITID,
			"jabatan":       "Staff",
			"gaji_pokok":    4000000,
			"tanggal_masuk": "2022-03-10",
		}
		createRec := doCreateKaryawanRequest(t, createBody)
		if createRec.Code != http.StatusCreated {
			t.Fatalf("setup: create karyawan harus 201, got %d, body: %s", createRec.Code, createRec.Body.String())
		}
		id := extractIDFromResponse(t, createRec)

		// Update: pindah departemen, naikkan jabatan & gaji.
		updateBody := map[string]interface{}{
			"nip":           "TUK-SUKSES-001", // NIP tidak berubah
			"nama":          "Andi Wijaya",
			"departemen_id": seedDepartemenHRID, // pindah dari IT ke HR
			"jabatan":       "Staff Senior",
			"gaji_pokok":    6000000,
			"tanggal_masuk": "2022-03-10",
		}
		rec := doUpdateKaryawanRequest(t, id, updateBody)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d, body: %s", rec.Code, rec.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("gagal decode response body: %v", err)
		}
		if resp["jabatan"] != "Staff Senior" {
			t.Errorf("expected jabatan Staff Senior, got %v", resp["jabatan"])
		}
		if int(resp["departemen_id"].(float64)) != seedDepartemenHRID {
			t.Errorf("expected departemen_id %d, got %v", seedDepartemenHRID, resp["departemen_id"])
		}
	})

	t.Run("tidak_ditemukan_404", func(t *testing.T) {
		updateBody := map[string]interface{}{
			"nip":           "TUK-404-001",
			"nama":          "Karyawan Tidak Ada",
			"departemen_id": seedDepartemenITID,
			"jabatan":       "Staff",
			"gaji_pokok":    3000000,
			"tanggal_masuk": "2022-01-01",
		}
		rec := doUpdateKaryawanRequest(t, 999999, updateBody)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d, body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("validasi_gagal_400_nip_kosong", func(t *testing.T) {
		// Setup: buat karyawan dulu supaya id-nya valid (biar gagalnya
		// murni karena validasi, bukan ketiban 404 duluan).
		createBody := map[string]interface{}{
			"nip":           "TUK-VALIDASI-001",
			"nama":          "Karyawan Untuk Update Invalid",
			"departemen_id": seedDepartemenITID,
			"jabatan":       "Staff",
			"gaji_pokok":    3000000,
			"tanggal_masuk": "2022-01-01",
		}
		createRec := doCreateKaryawanRequest(t, createBody)
		if createRec.Code != http.StatusCreated {
			t.Fatalf("setup: create karyawan harus 201, got %d, body: %s", createRec.Code, createRec.Body.String())
		}
		id := extractIDFromResponse(t, createRec)

		updateBody := map[string]interface{}{
			"nip":           "", // sengaja kosong
			"nama":          "Karyawan Untuk Update Invalid",
			"departemen_id": seedDepartemenITID,
			"jabatan":       "Staff",
			"gaji_pokok":    3000000,
			"tanggal_masuk": "2022-01-01",
		}
		rec := doUpdateKaryawanRequest(t, id, updateBody)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d, body: %s", rec.Code, rec.Body.String())
		}
	})
}

// doUpdateKaryawanRequest adalah helper untuk mengirim PUT /api/karyawan/:id
// lewat testRouter dan mengembalikan response recorder-nya.
func doUpdateKaryawanRequest(t *testing.T, id int, body map[string]interface{}) *httptest.ResponseRecorder {
	t.Helper()

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("gagal marshal request body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/karyawan/%d", id), bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	return rec
}

// extractIDFromResponse mengambil field "id" dari response body JSON hasil
// Create, dipakai berulang oleh test yang butuh id entity yang baru dibuat.
func extractIDFromResponse(t *testing.T, rec *httptest.ResponseRecorder) int {
	t.Helper()

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("gagal decode response body: %v", err)
	}
	idFloat, ok := resp["id"].(float64)
	if !ok {
		t.Fatalf("field id tidak ada atau bukan angka di response: %v", resp)
	}
	return int(idFloat)
}
