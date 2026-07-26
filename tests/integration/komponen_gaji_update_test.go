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

	// Test karyawan nonaktif (harus gagal).
	t.Run("karyawan_nonaktif_400", func(t *testing.T) {
		// Setup: buat karyawan aktif.
		createBody := map[string]interface{}{
			"nip":           "TKGUPDATE-002",
			"nama":          "Budi Hartono",
			"departemen_id": seedDepartemenITID,
			"jabatan":       "Programmer",
			"gaji_pokok":    5000000,
			"tanggal_masuk": "2023-01-01",
		}

		createRec := doCreateKaryawanRequest(t, createBody)
		if createRec.Code != http.StatusCreated {
			t.Fatalf("setup create karyawan gagal: %s",
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

		// Setup: soft delete karyawan.
		deleteReq := httptest.NewRequest(
			http.MethodDelete,
			fmt.Sprintf("/api/karyawan/%d", karyawanID),
			nil,
		)

		deleteRec := httptest.NewRecorder()
		testRouter.ServeHTTP(deleteRec, deleteReq)

		if deleteRec.Code != http.StatusOK {
			t.Fatalf("setup soft delete gagal: %s",
				deleteRec.Body.String())
		}

		// Request update.
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

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d, body: %s",
				rec.Code,
				rec.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("gagal decode response: %v", err)
		}

		if resp["error"] != "karyawan tidak aktif" {
			t.Errorf("expected error 'karyawan tidak aktif', got %v",
				resp["error"])
		}

		// Verifikasi database tidak berubah.
		var (
			jenis string
			nama  string
		)

		err := testPool.QueryRow(
			context.Background(),
			`SELECT jenis, nama
			 FROM komponen_gaji
			 WHERE id=$1`,
			komponenID,
		).Scan(&jenis, &nama)

		if err != nil {
			t.Fatalf("gagal query database: %v", err)
		}

		if jenis != "tunjangan" {
			t.Errorf("expected db jenis tetap 'tunjangan', got %s",
				jenis)
		}

		if nama != "Transport" {
			t.Errorf("expected db nama tetap 'Transport', got %s",
				nama)
		}
	})

	// Test: nama komponen gaji kosong, harus 400.
	t.Run("nama_kosong_400", func(t *testing.T) {
		// Setup: buat karyawan aktif.
		createBody := map[string]interface{}{
			"nip":           "TKGUPDATE-003",
			"nama":          "Candra Wijaya",
			"departemen_id": seedDepartemenITID,
			"jabatan":       "Staff",
			"gaji_pokok":    4500000,
			"tanggal_masuk": "2024-02-01",
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

		// Request update.
		updateBody := map[string]interface{}{
			"jenis":     "potongan",
			"nama":      "",
			"nominal":   150000,
			"is_persen": false,
		}

		rec := doUpdateKomponenGajiRequest(
			t,
			karyawanID,
			komponenID,
			updateBody,
		)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d, body: %s",
				rec.Code,
				rec.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("gagal decode response: %v", err)
		}

		if resp["error"] != "input tidak lengkap: pastikan semua field wajib sudah diisi" {
			t.Errorf("unexpected error message: %v", resp["error"])
		}

		// Verifikasi database tidak berubah.
		var (
			jenis string
			nama  string
		)

		err := testPool.QueryRow(
			context.Background(),
			`SELECT jenis, nama
			 FROM komponen_gaji
			 WHERE id=$1`,
			komponenID,
		).Scan(&jenis, &nama)

		if err != nil {
			t.Fatalf("gagal query database: %v", err)
		}

		if jenis != "tunjangan" {
			t.Errorf("expected db jenis tetap 'tunjangan', got %s",
				jenis)
		}

		if nama != "Transport" {
			t.Errorf("expected db nama tetap 'Transport', got %s",
				nama)
		}
	})

	// Test: jenis komponen gaji tidak valid, harus 400.
	t.Run("jenis_tidak_valid_400", func(t *testing.T) {
		// Setup: buat karyawan aktif.
		createBody := map[string]interface{}{
			"nip":           "TKGUPDATE-004",
			"nama":          "Dewi Anggraini",
			"departemen_id": seedDepartemenITID,
			"jabatan":       "Staff",
			"gaji_pokok":    4500000,
			"tanggal_masuk": "2024-03-01",
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

		// Request update.
		updateBody := map[string]interface{}{
			"jenis":     "bonus",
			"nama":      "Bonus Tahunan",
			"nominal":   1000000,
			"is_persen": false,
		}

		rec := doUpdateKomponenGajiRequest(
			t,
			karyawanID,
			komponenID,
			updateBody,
		)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d, body: %s",
				rec.Code,
				rec.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("gagal decode response: %v", err)
		}

		if resp["error"] != "jenis komponen gaji harus 'tunjangan' atau 'potongan'" {
			t.Errorf("unexpected error message: %v", resp["error"])
		}

		// Verifikasi database tidak berubah.
		var (
			jenis string
			nama  string
		)

		err := testPool.QueryRow(
			context.Background(),
			`SELECT jenis, nama
			 FROM komponen_gaji
			 WHERE id=$1`,
			komponenID,
		).Scan(&jenis, &nama)

		if err != nil {
			t.Fatalf("gagal query database: %v", err)
		}

		if jenis != "tunjangan" {
			t.Errorf("expected db jenis tetap 'tunjangan', got %s",
				jenis)
		}

		if nama != "Transport" {
			t.Errorf("expected db nama tetap 'Transport', got %s",
				nama)
		}
	})

	// Test: nominal negatif, harus 400.
	t.Run("nominal_negatif_400", func(t *testing.T) {
		// Setup: buat karyawan aktif.
		createBody := map[string]interface{}{
			"nip":           "TKGUPDATE-005",
			"nama":          "Dewi Anggraini",
			"departemen_id": seedDepartemenITID,
			"jabatan":       "Staff",
			"gaji_pokok":    4500000,
			"tanggal_masuk": "2024-03-01",
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

		// Request update.
		updateBody := map[string]interface{}{
			"jenis":     "potongan",
			"nama":      "BPJS",
			"nominal":   -9100,
			"is_persen": false,
		}

		rec := doUpdateKomponenGajiRequest(
			t,
			karyawanID,
			komponenID,
			updateBody,
		)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d, body: %s",
				rec.Code,
				rec.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("gagal decode response: %v", err)
		}

		if resp["error"] != "nominal tidak boleh negatif" {
			t.Errorf("unexpected error message: %v", resp["error"])
		}

		// Verifikasi database tidak berubah.
		var (
			jenis string
			nama  string
		)

		err := testPool.QueryRow(
			context.Background(),
			`SELECT jenis, nama
			 FROM komponen_gaji
			 WHERE id=$1`,
			komponenID,
		).Scan(&jenis, &nama)

		if err != nil {
			t.Fatalf("gagal query database: %v", err)
		}

		if jenis != "tunjangan" {
			t.Errorf("expected db jenis tetap 'tunjangan', got %s",
				jenis)
		}

		if nama != "Transport" {
			t.Errorf("expected db nama tetap 'Transport', got %s",
				nama)
		}
	})

	// Test: komponen gaji tidak ditemukan.
	t.Run("komponen_tidak_ditemukan_404", func(t *testing.T) {
		// Setup: buat karyawan aktif.
		createBody := map[string]interface{}{
			"nip":           "TKGUPDATE-006",
			"nama":          "Fajar Nugroho",
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

		updateBody := map[string]interface{}{
			"jenis":     "potongan",
			"nama":      "BPJS",
			"nominal":   150000,
			"is_persen": false,
		}

		rec := doUpdateKomponenGajiRequest(
			t,
			karyawanID,
			999999,
			updateBody,
		)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d, body=%s",
				rec.Code,
				rec.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		if resp["error"] != "komponen gaji tidak ditemukan" {
			t.Errorf("unexpected error message: %v",
				resp["error"])
		}
	})

	// Test: karyawan tidak ditemukan.
	t.Run("karyawan_tidak_ditemukan_404", func(t *testing.T) {
		// Setup: buat karyawan aktif.
		createBody := map[string]interface{}{
			"nip":           "TKGUPDATE-007",
			"nama":          "Galih Pratama",
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

		updateBody := map[string]interface{}{
			"jenis":     "potongan",
			"nama":      "BPJS",
			"nominal":   150000,
			"is_persen": false,
		}

		rec := doUpdateKomponenGajiRequest(
			t,
			999999,
			komponenID,
			updateBody,
		)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d body=%s",
				rec.Code,
				rec.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		if resp["error"] != "karyawan_id tidak valid atau tidak ditemukan" {
			t.Errorf("unexpected error message: %v",
				resp["error"])
		}

		// Verifikasi database tidak berubah.
		var (
			jenis string
			nama  string
		)

		err := testPool.QueryRow(
			context.Background(),
			`SELECT jenis, nama
			 FROM komponen_gaji
			 WHERE id=$1`,
			komponenID,
		).Scan(&jenis, &nama)

		if err != nil {
			t.Fatalf("query db gagal: %v", err)
		}

		if jenis != "tunjangan" {
			t.Errorf("expected db jenis tetap 'tunjangan', got %s",
				jenis)
		}

		if nama != "Transport" {
			t.Errorf("expected db nama tetap 'Transport', got %s",
				nama)
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
