package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shopspring/decimal"
)

// TestGetKomponenGajiByID menguji endpoint
// GET /api/komponen-gaji/:id.
func TestGetKomponenGajiByID(t *testing.T) {
	t.Run("sukses_200", func(t *testing.T) {

		// Setup: buat karyawan.
		createBody := map[string]interface{}{
			"nip":           "TKGGETID-001",
			"nama":          "Rahmat Hidayat",
			"departemen_id": seedDepartemenITID,
			"jabatan":       "Staff",
			"gaji_pokok":    5000000,
			"tanggal_masuk": "2024-01-01",
		}

		createRec := doCreateKaryawanRequest(t, createBody)
		if createRec.Code != http.StatusCreated {
			t.Fatalf("setup create karyawan gagal: %d body=%s",
				createRec.Code,
				createRec.Body.String())
		}

		karyawanID := extractIDFromResponse(t, createRec)

		// Setup: buat komponen gaji.
		komponenBody := map[string]interface{}{
			"jenis":     "tunjangan",
			"nama":      "Transport",
			"nominal":   300000,
			"is_persen": false,
		}

		komponenRec := doCreateKomponenGajiRequest(
			t,
			karyawanID,
			komponenBody,
		)

		if komponenRec.Code != http.StatusCreated {
			t.Fatalf("setup create komponen gagal: %d body=%s",
				komponenRec.Code,
				komponenRec.Body.String())
		}

		komponenID := extractIDFromResponse(t, komponenRec)

		rec := doGetKomponenGajiByIDRequest(t, komponenID)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s",
				rec.Code,
				rec.Body.String())
		}

		var resp map[string]interface{}

		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response gagal: %v", err)
		}

		if int(resp["id"].(float64)) != komponenID {
			t.Errorf("expected id %d got %v",
				komponenID,
				resp["id"])
		}

		if int(resp["karyawan_id"].(float64)) != karyawanID {
			t.Errorf("expected karyawan_id %d got %v",
				karyawanID,
				resp["karyawan_id"])
		}

		if resp["jenis"] != "tunjangan" {
			t.Errorf("expected jenis=tunjangan got %v",
				resp["jenis"])
		}

		if resp["nama"] != "Transport" {
			t.Errorf("expected nama Transport got %v",
				resp["nama"])
		}

		nominal := parseDecimalField(t, resp, "nominal")

		if !nominal.Equal(decimal.NewFromInt(300000)) {
			t.Errorf("expected nominal 300000 got %s",
				nominal)
		}

		if resp["is_persen"] != false {
			t.Errorf("expected is_persen=false")
		}
	})

	// Test: komponen_tidak_ditemukan
	t.Run("komponen_tidak_ditemukan_404", func(t *testing.T) {

		// Gunakan ID yang dipastikan tidak ada.
		rec := doGetKomponenGajiByIDRequest(t, 999999)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d body=%s",
				rec.Code,
				rec.Body.String())
		}

		var resp map[string]interface{}

		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response gagal: %v", err)
		}

		if resp["error"] != "komponen gaji tidak ditemukan" {
			t.Errorf("unexpected error message: %v",
				resp["error"])
		}
	})

	// Test: id bukan angka
	t.Run("id_bukan_angka_400", func(t *testing.T) {

		req := httptest.NewRequest(
			http.MethodGet,
			"/api/komponen-gaji/a<!",
			nil,
		)

		rec := httptest.NewRecorder()
		testRouter.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%s",
				rec.Code,
				rec.Body.String())
		}

		var resp map[string]interface{}

		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response gagal: %v", err)
		}

		if resp["error"] != "id harus berupa angka" {
			t.Errorf("unexpected error message: %v",
				resp["error"])
		}
	})

}

// doGetKomponenGajiByIDRequest mengirim
// GET /api/komponen-gaji/:id.
func doGetKomponenGajiByIDRequest(
	t *testing.T,
	komponenID int,
) *httptest.ResponseRecorder {

	t.Helper()

	req := httptest.NewRequest(
		http.MethodGet,
		fmt.Sprintf("/api/komponen-gaji/%d", komponenID),
		nil,
	)

	rec := httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	return rec
}
