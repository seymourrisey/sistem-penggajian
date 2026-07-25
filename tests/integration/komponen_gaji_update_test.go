package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestUpdateKomponenGaji menguji endpoint
// PUT /api/karyawan/:id/komponen-gaji/:komponen_id.
func TestUpdateKomponenGaji(t *testing.T) {

	t.Run("sukses_200", func(t *testing.T) {
		// Setup: buat karyawan aktif.
		createBody := map[string]interface{}{
			"nip":           "TKGUPDATE-001",
			"nama":          "Ahmad Fauzi",
			"departemen_id": seedDepartemenITID,
			"jabatan":       "Staff",
			"gaji_pokok":    5000000,
			"tanggal_masuk": "2024-01-01",
		}

		createRec := doCreateKaryawanRequest(t, createBody)
		if createRec.Code != http.StatusCreated {
			t.Fatalf("setup create karyawan gagal: %d %s",
				createRec.Code,
				createRec.Body.String())
		}

		karyawanID := extractIDFromResponse(t, createRec)

		// Setup: buat komponen gaji.
		createKomponen := map[string]interface{}{
			"jenis":     "tunjangan",
			"nama":      "Transport",
			"nominal":   300000,
			"is_persen": false,
		}

		komponenRec := doCreateKomponenGajiRequest(
			t,
			karyawanID,
			createKomponen,
		)

		if komponenRec.Code != http.StatusCreated {
			t.Fatalf("setup create komponen gagal: %d %s",
				komponenRec.Code,
				komponenRec.Body.String())
		}

		komponenID := extractIDFromResponse(t, komponenRec)

		// Update.
		updateBody := map[string]interface{}{
			"jenis":     "potongan",
			"nama":      "BPJS",
			"nominal":   150000,
			"is_persen": false,
		}

		rec := doUpdateKomponenGajiRequest(
			t,
			karyawanID,
			komponenID,
			updateBody,
		)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s",
				rec.Code,
				rec.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		if resp["jenis"] != "potongan" {
			t.Errorf("expected jenis=potongan got %v", resp["jenis"])
		}

		if resp["nama"] != "BPJS" {
			t.Errorf("expected nama BPJS got %v", resp["nama"])
		}

		if resp["nominal"] != "150000" {
			t.Errorf("expected nominal 150000 got %v", resp["nominal"])
		}

		if resp["is_persen"] != false {
			t.Errorf("expected is_persen=false")
		}

		// Verifikasi database.
		var (
			jenis string
			nama  string
		)

		err := testPool.QueryRow(
			context.Background(),
			`SELECT jenis,nama
			 FROM komponen_gaji
			 WHERE id=$1`,
			komponenID,
		).Scan(&jenis, &nama)

		if err != nil {
			t.Fatalf("query db gagal: %v", err)
		}

		if jenis != "potongan" {
			t.Errorf("expected db jenis potongan got %s", jenis)
		}

		if nama != "BPJS" {
			t.Errorf("expected db nama BPJS got %s", nama)
		}
	})
}

// doUpdateKomponenGajiRequest mengirim
// PUT /api/karyawan/:id/komponen-gaji/:komponen_id.
func doUpdateKomponenGajiRequest(
	t *testing.T,
	karyawanID int,
	komponenID int,
	body map[string]interface{},
) *httptest.ResponseRecorder {

	t.Helper()

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodPut,
		fmt.Sprintf(
			"/api/karyawan/%d/komponen-gaji/%d",
			karyawanID,
			komponenID,
		),
		bytes.NewReader(payload),
	)

	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	return rec
}
