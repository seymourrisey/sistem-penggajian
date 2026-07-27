package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

const notExistingID = 999999

// TestCreateKaryawan menguji POST /api/karyawan lewat 4 skenario: sukses
// (201), validasi input gagal (400), NIP duplikat (409, uji UNIQUE
// constraint), dan departemen_id yang tidak ada (400, uji FK constraint
// via mapKaryawanError -> repository.ErrDepartemenTidakValid).
func TestCreateKaryawan(t *testing.T) {
	t.Run("sukses_201", func(t *testing.T) {
		body := map[string]interface{}{
			"nip":           "TCK-SUKSES-001",
			"nama":          "Budi Santoso",
			"departemen_id": seedDepartemenITID,
			"jabatan":       "Staff",
			"gaji_pokok":    5000000,
			"tanggal_masuk": "2024-01-15",
		}
		rec := doCreateKaryawanRequest(t, body)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d, body: %s", rec.Code, rec.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("gagal decode response body: %v", err)
		}
		if resp["nip"] != "TCK-SUKSES-001" {
			t.Errorf("expected nip TCK-SUKSES-001, got %v", resp["nip"])
		}
		if resp["status"] != "aktif" {
			t.Errorf("expected status aktif (default karyawan baru), got %v", resp["status"])
		}
		if _, ok := resp["id"]; !ok {
			t.Errorf("expected field id ada di response, tidak ditemukan")
		}
	})

	t.Run("validasi_gagal_400_nama_kosong", func(t *testing.T) {
		body := map[string]interface{}{
			"nip":           "TCK-VALIDASI-001",
			"nama":          "", // sengaja kosong, binding:"required" harus menolak
			"departemen_id": seedDepartemenITID,
			"jabatan":       "Staff",
			"gaji_pokok":    5000000,
			"tanggal_masuk": "2024-01-15",
		}
		rec := doCreateKaryawanRequest(t, body)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d, body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("nip_duplikat_409", func(t *testing.T) {
		body := map[string]interface{}{
			"nip":           "TCK-DUPLIKAT-001",
			"nama":          "Karyawan Pertama",
			"departemen_id": seedDepartemenITID,
			"jabatan":       "Staff",
			"gaji_pokok":    5000000,
			"tanggal_masuk": "2024-01-15",
		}

		// Insert pertama harus sukses.
		recFirst := doCreateKaryawanRequest(t, body)
		if recFirst.Code != http.StatusCreated {
			t.Fatalf("setup: insert pertama harus 201, got %d, body: %s", recFirst.Code, recFirst.Body.String())
		}

		// Insert kedua dengan NIP sama harus gagal 409.
		body["nama"] = "Karyawan Kedua Nama Beda" // nama beda, NIP sama — tetap harus ditolak
		recSecond := doCreateKaryawanRequest(t, body)
		if recSecond.Code != http.StatusConflict {
			t.Fatalf("expected status 409 untuk NIP duplikat, got %d, body: %s", recSecond.Code, recSecond.Body.String())
		}
	})

	t.Run("departemen_id_invalid_400", func(t *testing.T) {
		body := map[string]interface{}{
			"nip":           "TCK-DEPTINVALID-001",
			"nama":          "Karyawan Departemen Salah",
			"departemen_id": notExistingID, // sengaja tidak ada di tabel departemen
			"jabatan":       "Staff",
			"gaji_pokok":    5000000,
			"tanggal_masuk": "2024-01-15",
		}
		rec := doCreateKaryawanRequest(t, body)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400 untuk departemen_id tidak valid, got %d, body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("gaji_pokok_0_400", func(t *testing.T) {
		body := map[string]interface{}{
			"nip":           "TCK-GAJINOL-001",
			"nama":          "Budi Santoso",
			"departemen_id": seedDepartemenITID,
			"jabatan":       "Staff",
			"gaji_pokok":    0,
			"tanggal_masuk": "2024-01-15",
		}

		rec := doCreateKaryawanRequest(t, body)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d, body: %s",
				rec.Code,
				rec.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("gagal decode response: %v", err)
		}

		if resp["error"] != "gaji pokok tidak boleh nol" {
			t.Errorf("expected error 'gaji pokok tidak boleh nol', got %v",
				resp["error"])
		}
	})

	t.Run("gaji_pokok_negatif_400", func(t *testing.T) {
		body := map[string]interface{}{
			"nip":           "TCK-GAJINEG-002",
			"nama":          "Budi Santoso",
			"departemen_id": seedDepartemenITID,
			"jabatan":       "Staff",
			"gaji_pokok":    -1,
			"tanggal_masuk": "2024-01-15",
		}

		rec := doCreateKaryawanRequest(t, body)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d, body: %s",
				rec.Code,
				rec.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("gagal decode response: %v", err)
		}

		if resp["error"] != "gaji pokok tidak boleh negatif" {
			t.Errorf("expected error 'gaji pokok tidak boleh negatif', got %v",
				resp["error"])
		}
	})
}

// doCreateKaryawanRequest adalah helper untuk mengirim POST /api/karyawan
// lewat testRouter dan mengembalikan response recorder-nya. Dipakai
// berulang oleh seluruh subtest TestCreateKaryawan.
func doCreateKaryawanRequest(t *testing.T, body map[string]interface{}) *httptest.ResponseRecorder {
	t.Helper()

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("gagal marshal request body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/karyawan", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	return rec
}
