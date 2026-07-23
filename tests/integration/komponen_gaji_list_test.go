package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shopspring/decimal"
)

// TestGetKomponenGajiByKaryawanID menguji GET /api/karyawan/:id/komponen-gaji,
// khususnya memverifikasi bahwa hasil diurutkan berdasarkan nominal terbesar
// menggunakan algoritma sorting manual di internal/util/sort.go (F7, KUK
// unit #4) — bukan sekadar urutan insert atau ORDER BY SQL.
func TestGetKomponenGajiByKaryawanID(t *testing.T) {
	t.Run("sukses_200_terurut_nominal_desc", func(t *testing.T) {
		// Setup: karyawan baru.
		createBody := map[string]interface{}{
			"nip":           "TKG-SORT-001",
			"nama":          "Fajar Nugroho",
			"departemen_id": seedDepartemenITID,
			"jabatan":       "Staff",
			"gaji_pokok":    4000000,
			"tanggal_masuk": "2023-05-01",
		}
		createRec := doCreateKaryawanRequest(t, createBody)
		if createRec.Code != http.StatusCreated {
			t.Fatalf("setup: create karyawan harus 201, got %d, body: %s", createRec.Code, createRec.Body.String())
		}
		karyawanID := extractIDFromResponse(t, createRec)

		// Sengaja diinsert TIDAK berurutan, supaya hasil urut membuktikan
		// sorting benar-benar terjadi (bukan kebetulan sama dengan urutan insert).
		komponenList := []map[string]interface{}{
			{"jenis": "tunjangan", "nama": "Tunjangan Transport", "nominal": 250000, "is_persen": false},
			{"jenis": "tunjangan", "nama": "Tunjangan Makan", "nominal": 500000, "is_persen": false},
			{"jenis": "potongan", "nama": "BPJS Kesehatan", "nominal": 100000, "is_persen": false},
			{"jenis": "tunjangan", "nama": "Tunjangan Jabatan", "nominal": 350000, "is_persen": false},
		}
		for _, body := range komponenList {
			rec := doCreateKomponenGajiRequest(t, karyawanID, body)
			if rec.Code != http.StatusCreated {
				t.Fatalf("setup: create komponen %q harus 201, got %d, body: %s", body["nama"], rec.Code, rec.Body.String())
			}
		}

		// Expected urutan nominal descending: 500000, 350000, 250000, 100000.
		expectedNominal := []decimal.Decimal{
			decimal.NewFromInt(500000),
			decimal.NewFromInt(350000),
			decimal.NewFromInt(250000),
			decimal.NewFromInt(100000),
		}

		rec := doGetKomponenGajiByKaryawanIDRequest(t, karyawanID)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d, body: %s", rec.Code, rec.Body.String())
		}

		var resp []map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("gagal decode response body: %v", err)
		}
		if len(resp) != len(expectedNominal) {
			t.Fatalf("expected %d komponen, got %d: %v", len(expectedNominal), len(resp), resp)
		}

		for i, expected := range expectedNominal {
			actual := parseDecimalField(t, resp[i], "nominal")
			if !actual.Equal(expected) {
				t.Errorf("posisi ke-%d: expected nominal %s, got %s (nama: %v)", i, expected, actual, resp[i]["nama"])
			}
		}
	})

	t.Run("kosong_200_array_kosong", func(t *testing.T) {
		// Setup: karyawan baru, belum ada komponen gaji sama sekali.
		createBody := map[string]interface{}{
			"nip":           "TKG-KOSONG-001",
			"nama":          "Gita Ramadhani",
			"departemen_id": seedDepartemenITID,
			"jabatan":       "Staff",
			"gaji_pokok":    3000000,
			"tanggal_masuk": "2023-06-01",
		}
		createRec := doCreateKaryawanRequest(t, createBody)
		if createRec.Code != http.StatusCreated {
			t.Fatalf("setup: create karyawan harus 201, got %d, body: %s", createRec.Code, createRec.Body.String())
		}
		karyawanID := extractIDFromResponse(t, createRec)

		rec := doGetKomponenGajiByKaryawanIDRequest(t, karyawanID)

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
}

// doGetKomponenGajiByKaryawanIDRequest adalah helper untuk mengirim
// GET /api/karyawan/:id/komponen-gaji lewat testRouter.
func doGetKomponenGajiByKaryawanIDRequest(t *testing.T, karyawanID int) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/karyawan/%d/komponen-gaji", karyawanID), nil)
	rec := httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	return rec
}
