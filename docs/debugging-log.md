# Debugging Log — Sistem Informasi Penggajian

**Bukti Kompetensi:** #6 — Melakukan Debugging
**Metode:** Exploratory testing manual via Postman terhadap seluruh endpoint REST API, dilakukan setelah handler + router + wiring selesai
**Total kasus:** 13 bug ditemukan, 13 bug diperbaiki dan diverifikasi ulang.

---

## Cara Membaca Dokumen Ini

Setiap kasus dicatat dengan struktur yang sama:
- **Endpoint** — endpoint HTTP yang terdampak (atau keterangan bahwa bug murni di test/infra, tidak ada endpoint HTTP langsung).
- **Severity** — Low/Medium/High, skala yang sama dipakai konsisten di seluruh dokumen ini (dan di-reuse di `docs/code-review-checklist.md`).
- **Langkah Reproduksi** — request persis yang memicu bug.
- **Before** — response/behavior sebelum fix (bukti bug nyata terjadi, bukan hipotesis).
- **Root Cause** — analisis kenapa bug terjadi, bukan cuma gejalanya.
- **Fix (After)** — perubahan kode yang menyelesaikan root cause.
- **Verifikasi** — hasil retest setelah fix diterapkan, bukti bug sudah tidak muncul lagi.

---

## Bug #1 — Delete Departemen yang Masih Direferensikan Karyawan Menghasilkan 500, Bukan 409

- **Endpoint:** `DELETE /api/departemen/:id`
- **Severity:** Medium (kesalahan HTTP semantic — client error dianggap server error)

### Langkah Reproduksi
```
DELETE /api/departemen/1
```
(Departemen ID 1 masih punya minimal 1 karyawan yang mereferensikannya)

### Before
```
Status: 500 Internal Server Error
```
```json
{
  "error": "gagal hapus departemen id=1: ERROR: update or delete on table \"departemen\" violates foreign key constraint \"karyawan_departemen_id_fkey\" on table \"karyawan\" (SQLSTATE 23503)"
}
```

### Root Cause
`departemenRepository.Delete` tidak melakukan mapping terhadap Postgres error code — semua error dari `Exec` langsung dibungkus generik dengan `fmt.Errorf`, termasuk pelanggaran foreign key constraint (kode `23503`). Akibatnya, kesalahan yang sebetulnya disebabkan oleh input/state yang tidak valid dari sisi client (mencoba hapus resource yang masih dipakai) diklasifikasikan sebagai kegagalan server (500), dan pesan error mentah PostgreSQL (termasuk nama constraint internal) bocor ke response API.

### Fix (After)
Tambah sentinel error baru dan mapping pgcode di `internal/repository/departemen_repository.go`:
```go
var ErrDepartemenMasihDipakai = errors.New("departemen tidak dapat dihapus karena masih direferensikan karyawan")

func (r *departemenRepository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM departemen WHERE id = $1`
	cmdTag, err := r.db.Exec(ctx, query, id)
	if err != nil {
		if mapped := mapDepartemenPgError(err); mapped != nil {
			return mapped
		}
		return fmt.Errorf("gagal hapus departemen id=%d: %w", id, err)
	}
	if cmdTag.RowsAffected() == 0 {
		return ErrDepartemenNotFound
	}
	return nil
}

func mapDepartemenPgError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return ErrDepartemenNamaSudahAda
		case "23503":
			return ErrDepartemenMasihDipakai
		}
	}
	return nil
}
```
Handler layer (`mapDepartemenError`) menambahkan case untuk sentinel baru ini → HTTP 409.

### Verifikasi
```
DELETE /api/departemen/1
```
```
Status: 409 Conflict
```
```json
{
  "error": "departemen tidak dapat dihapus karena masih direferensikan karyawan"
}
```
Confirmed via Postman, log server: `[GIN] ... | 409 | ... | DELETE "/api/departemen/1"`.

---

## Bug #2, #3, #4, #5 — Pesan Error Mentah dari Library Bocor ke Client (4 Kasus, 1 Root Cause)

Empat kasus di bawah ini digabung karena akar masalahnya identik: seluruh handler mengirim `err.Error()` langsung dari `c.ShouldBindJSON()` ke response client tanpa translasi, sehingga pesan internal Go/library bocor ke API publik.

- **Endpoint terdampak:** semua endpoint yang menerima JSON body (`POST`/`PUT` di departemen, karyawan, komponen-gaji, payroll)
- **Severity:** Medium (bukan security-critical, tapi API contract rusak — pesan tidak konsisten bahasa Indonesia, membocorkan nama struct internal)

### Kasus #2 — Field Required Kosong (string `""`)

**Langkah Reproduksi**
```
POST /api/departemen
{"nama": ""}
```

**Before**
```
Status: 400 Bad Request
```
```json
{
  "error": "Key: 'departemenCreateRequest.Nama' Error:Field validation for 'Nama' failed on the 'required' tag"
}
```

### Kasus #3 — Field Numerik Dikirim Sebagai String Kosong

**Langkah Reproduksi**
```
POST /api/karyawan
{"nip":"", "nama":"", "departemen_id":"", "jabatan":"", "gaji_pokok":"", "tanggal_masuk":""}
```

**Before**
```
Status: 400 Bad Request
```
```json
{
  "error": "error decoding string '\"\"': can't convert \"\" to decimal"
}
```

### Kasus #4 — Type Mismatch pada Field JSON

**Langkah Reproduksi**
```
POST /api/karyawan
{"nip":"TESTXXX", "nama":"", "departemen_id":"<>a", "jabatan":"1241", "gaji_pokok":"9999", "tanggal_masuk":"2026-03-03"}
```

**Before**
```
Status: 400 Bad Request
```
```json
{
  "error": "json: cannot unmarshal string into Go struct field karyawanCreateRequest.departemen_id of type int"
}
```

### Kasus #5 — Partial Update Memicu Validator Error Bertumpuk

**Langkah Reproduksi**
```
PUT /api/karyawan/1
{"nama": "Tom Pearl"}
```

**Before**
```
Status: 400 Bad Request
```
```json
{
  "error": "Key: 'karyawanUpdateRequest.NIP' Error:Field validation for 'NIP' failed on the 'required' tag\nKey: 'karyawanUpdateRequest.DepartemenID' Error:Field validation for 'DepartemenID' failed on the 'required' tag\nKey: 'karyawanUpdateRequest.Jabatan' Error:Field validation for 'Jabatan' failed on the 'required' tag\nKey: 'karyawanUpdateRequest.TanggalMasuk' Error:Field validation for 'TanggalMasuk' failed on the 'required' tag"
}
```

### Root Cause (untuk keempat kasus)
Semua handler menulis pola yang sama:
```go
if err := c.ShouldBindJSON(&req); err != nil {
    c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
    return
}
```
`err` di sini bisa berasal dari 3 sumber berbeda tergantung jenis kegagalan — `validator.ValidationErrors` (tag `required` gagal), `*json.UnmarshalTypeError` (tipe data JSON tidak cocok dengan struct Go), atau error generik dari custom `UnmarshalJSON` milik tipe lain (`decimal.Decimal`) — dan ketiganya diteruskan mentah-mentah tanpa dibedakan atau diterjemahkan.

### Fix (After)
Dibuat satu helper terpusat di file baru `internal/handler/bind_error.go`, dipakai di seluruh handler (7 lokasi lintas 4 file):
```go
func translateBindError(err error) string {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		return "input tidak lengkap: pastikan semua field wajib sudah diisi"
	}

	var ute *json.UnmarshalTypeError
	if errors.As(err, &ute) {
		return fmt.Sprintf("field '%s' memiliki tipe data yang salah", ute.Field)
	}

	var se *json.SyntaxError
	if errors.As(err, &se) {
		return "format JSON tidak valid"
	}

	return "data yang dikirim tidak valid, periksa kembali format dan isi field"
}
```
Semua handler diubah dari `gin.H{"error": err.Error()}` menjadi `gin.H{"error": translateBindError(err)}` tepat setelah `ShouldBindJSON` gagal.

### Verifikasi
| Kasus | Response Setelah Fix |
|---|---|
| #2 (nama kosong) | `400` — `{"error": "input tidak lengkap: pastikan semua field wajib sudah diisi"}` |
| #3 (decimal kosong) | `400` — `{"error": "data yang dikirim tidak valid, periksa kembali format dan isi field"}` |
| #4 (type mismatch) | `400` — `{"error": "field 'departemen_id' memiliki tipe data yang salah"}` |
| #5 (partial update) | `400` — `{"error": "input tidak lengkap: pastikan semua field wajib sudah diisi"}` |

Keempat kasus dites ulang via Postman, semua konsisten bahasa Indonesia, tidak ada lagi pesan mentah library atau nama struct internal yang bocor.

---

## Bug #6 & #7 — Duplikat Komponen Gaji Bisa Tersimpan, Error Duplikat Saat Update Bocor Mentah (2 Kasus, 1 Root Cause)

- **Endpoint terdampak:** `POST /api/karyawan/:id/komponen-gaji`, `PUT /api/karyawan/:id/komponen-gaji/:komponen_id`
- **Severity:** Kasus #6 = High (data integrity issue — data invalid bisa tersimpan). Kasus #7 = Medium (error handling — pesan mentah bocor ke client, tapi tidak ada data yang corrupt). Konsisten dengan pemisahan severity di tabel Ringkasan.

### Kasus #6 — Insert Duplikat Diterima Tanpa Ditolak

**Langkah Reproduksi**
```
POST /api/karyawan/1/komponen-gaji
{"jenis": "tunjangan", "nama": "Transport", "nominal": 500000, "is_persen": false}
```
Dikirim 2 kali dengan payload identik.

**Before**
Kedua request berhasil (`201 Created`), menghasilkan 2 row berbeda di tabel `komponen_gaji` dengan `karyawan_id`, `jenis`, dan `nama` yang sama persis — data yang secara bisnis seharusnya tidak valid (satu karyawan tidak boleh punya 2 komponen "Transport" jenis "tunjangan").

### Kasus #7 — Error Duplikat Saat Update Bocor Mentah

**Langkah Reproduksi**
```
PUT /api/karyawan/1/komponen-gaji/5
{"jenis": "tunjangan", "nama": "Transport", "nominal": 500000, "is_persen": false}
```
(Karyawan 1 sudah punya komponen lain dengan jenis+nama yang sama)

**Before**
```
Status: 500 Internal Server Error
```
```json
{
  "error": "ERROR: duplicate key value violates unique constraint \"uq_komponen_gaji_karyawan_jenis_nama\" (SQLSTATE 23505)"
}
```

### Root Cause
Dua masalah terpisah yang saling berkaitan:
1. Tabel `komponen_gaji` di database aktual **belum punya UNIQUE constraint** pada kombinasi `(karyawan_id, jenis, nama)`, walaupun constraint ini sudah tercantum di skema desain (`project-design.md` section 2.1) — constraint di dokumen tidak pernah benar-benar diterapkan (dieksekusi) ke database.
2. Fungsi `komponenGajiRepository.Update` tidak pernah memanggil `mapKomponenGajiPgError` sama sekali (berbeda dari `Create` yang sejak awal sudah benar memanggilnya) — begitu constraint di poin 1 ditambahkan, error 23505 dari `Update` tetap bocor mentah karena tidak ada mapping.

### Fix (After)
**Skema database** (dijalankan manual via pgAdmin4):
```sql
ALTER TABLE komponen_gaji
ADD CONSTRAINT uq_komponen_gaji_karyawan_jenis_nama
UNIQUE (karyawan_id, jenis, nama);
```

**Kode** — `internal/repository/komponen_gaji_repository.go`:
```go
var ErrKomponenGajiDuplikat = errors.New("komponen gaji dengan jenis dan nama ini sudah ada untuk karyawan tersebut")

func (r *komponenGajiRepository) Update(ctx context.Context, k *model.KomponenGaji) error {
	query := `
		UPDATE komponen_gaji
		SET jenis = $1, nama = $2, nominal = $3, is_persen = $4
		WHERE id = $5 AND karyawan_id = $6`

	tag, err := r.db.Exec(ctx, query, k.Jenis, k.Nama, k.Nominal, k.IsPersen, k.ID, k.KaryawanID)
	if err != nil {
		if mapped := mapKomponenGajiPgError(err); mapped != nil {
			return mapped
		}
		return fmt.Errorf("gagal update komponen gaji id=%d: %w", k.ID, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrKomponenGajiNotFound
	}
	return nil
}

func mapKomponenGajiPgError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23503":
			return ErrKaryawanTidakValid
		case "23505":
			return ErrKomponenGajiDuplikat
		}
	}
	return nil
}
```
Handler (`mapKomponenGajiError`) menambahkan case `ErrKomponenGajiDuplikat` → HTTP 409.

### Verifikasi
```
POST /api/karyawan/1/komponen-gaji
```
(payload identik, dikirim setelah 1 komponen sudah ada)
```
Status: 409 Conflict
```
```json
{
  "error": "komponen gaji dengan jenis dan nama ini sudah ada untuk karyawan tersebut"
}
```
Confirmed via Postman untuk kedua endpoint (`Create` dan `Update`), tidak ada lagi duplikat yang bisa tersimpan.

---

## Bug #8 — IDOR: Update Komponen Gaji Bisa Menembus Kepemilikan Karyawan Lain

- **Endpoint:** `PUT /api/karyawan/:id/komponen-gaji/:komponen_id`
- **Severity:** High — ini termasuk kategori **security flaw** (Insecure Direct Object Reference / IDOR), bukan sekadar bug fungsional.

### Langkah Reproduksi
```
PUT /api/karyawan/1/komponen-gaji/15
```
Dengan `komponen_id = 15` sebenarnya adalah milik `karyawan_id = 2`, bukan `karyawan_id = 1`.

### Before
Request tetap berhasil (`200 OK`) dan **benar-benar mengubah** data komponen gaji milik karyawan 2, meskipun URL menyebutkan `karyawan_id = 1`. Sistem tidak pernah memvalidasi bahwa komponen yang di-update benar-benar milik karyawan yang disebut di URL.

### Root Cause
Query `UPDATE` pada `komponenGajiRepository.Update` hanya memfilter berdasarkan `id` komponen (`WHERE id = $5`), sama sekali tidak memvalidasi `karyawan_id` dari URL path terhadap `karyawan_id` yang sebenarnya tersimpan di row tersebut. Akibatnya, `karyawan_id` di URL murni kosmetik — siapa pun yang tahu (atau menebak) `komponen_id` yang valid bisa mengubahnya lewat `karyawan_id` mana pun.

### Fix (After)
```go
query := `
    UPDATE komponen_gaji
    SET jenis = $1, nama = $2, nominal = $3, is_persen = $4
    WHERE id = $5 AND karyawan_id = $6`
```
Ditambahkan klausa `AND karyawan_id = $6` — kalau kombinasi `id` + `karyawan_id` tidak cocok (baik karena ID salah maupun karena komponen itu milik karyawan lain), `RowsAffected()` akan bernilai 0 dan fungsi mengembalikan `ErrKomponenGajiNotFound` (dipetakan ke 404), bukan diam-diam berhasil mengubah row yang salah.

### Verifikasi
```
PUT /api/karyawan/1/komponen-gaji/15
```
(dengan komponen_id=15 milik karyawan 2)
```
Status: 404 Not Found
```
```json
{
  "error": "komponen gaji tidak ditemukan"
}
```
Data milik karyawan 2 dikonfirmasi tidak berubah setelah request ini dikirim.

---

## Bug #9 — Response Ganda/Corrupt Saat Validasi PUT Departemen Gagal

- **Endpoint:** `PUT /api/departemen/:id`
- **Severity:** Medium — tidak menyebabkan data corruption, tapi API contract rusak (client menerima response tidak valid/ambigu).

### Langkah Reproduksi
```
PUT /api/departemen/6
{"nama": ""}
```

### Before
Server log menunjukkan warning `[GIN] [WARNING] Headers were already written`, dan response body berpotensi berisi output ganda/tidak konsisten (dua kali proses penulisan response ke connection yang sama).

### Root Cause
Fungsi `Update` di `departemen_handler.go` menggunakan `c.BindJSON(&req)`, bukan `c.ShouldBindJSON(&req)` seperti seluruh handler lain di project ini. `c.BindJSON` di Gin **otomatis menulis response 400 sendiri** (via `AbortWithError`) begitu validasi gagal — tapi kode di baris berikutnya tetap mengeksekusi `c.JSON(http.StatusBadRequest, ...)` lagi secara manual, menyebabkan response ditulis dua kali ke koneksi HTTP yang sama.

### Fix (After)
```go
// Sebelum:
if err := c.BindJSON(&req); err != nil {
    c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
    return
}

// Sesudah:
if err := c.ShouldBindJSON(&req); err != nil {
    c.JSON(http.StatusBadRequest, gin.H{"error": translateBindError(err)})
    return
}
```
`c.ShouldBindJSON` tidak menulis response otomatis — kontrol penuh tetap di tangan handler, konsisten dengan pola yang dipakai di semua handler lain.

### Verifikasi
```
PUT /api/departemen/6
{"nama": ""}
```
```
Status: 400 Bad Request
```
```json
{
  "error": "input tidak lengkap: pastikan semua field wajib sudah diisi"
}
```
Response bersih, satu JSON object tunggal, tidak ada lagi warning "headers already written" di server log.

---

## Bug #10 — Response JSON Tanggal Menggunakan RFC3339, Tidak Sesuai API Contract

- **Endpoint terdampak:**
  - `POST /api/payroll/generate`
  - `GET /api/payroll/:karyawan_id/riwayat`
  - `GET /api/payroll/laporan`
  - `POST /api/karyawan`
  - `GET /api/karyawan`
  - `GET /api/karyawan/:id`
  - `PUT /api/karyawan/:id`
- **Severity:** Medium

### Langkah Reproduksi
Lakukan request ke endpoint payroll atau karyawan, kemudian perhatikan field `periode` atau `tanggal_masuk` pada response.

### Before
```json
{"periode":"2026-07-20T00:00:00Z","tanggal_masuk":"2024-03-01T00:00:00Z"}
```

### Root Cause
Field bertipe `time.Time` dikirim langsung menggunakan `c.JSON(...)`. Library `encoding/json` memanggil `time.Time.MarshalJSON()` sehingga otomatis menghasilkan format RFC3339, bukan `YYYY-MM-DD` seperti yang ditetapkan pada API Contract.

### Fix (After)
Ditambahkan DTO (`karyawanResponse`, `payrollResponse`, `riwayatResponse`, dan `laporanResponse`) pada layer handler. Seluruh field tanggal diformat menggunakan `Format(dateOnlyLayout)` sebelum dikirim ke client. Model domain tetap menggunakan `time.Time` sehingga repository dan database tidak berubah.

### Verifikasi
```json
{"periode":"2026-07-20","tanggal_masuk":"2024-03-01"}
```
Seluruh endpoint terkait diuji ulang melalui Postman dan kini konsisten menggunakan format `YYYY-MM-DD`.

---

## Bug #11 — Mock PayrollRepository Tidak Sinkron Setelah Interface Berubah (Transaksi pgx)

- **Endpoint terdampak:** Tidak ada endpoint HTTP langsung — ini compile-time failure di `tests/unit/payroll_service_test.go`, dipicu oleh perubahan signature interface `PayrollRepository.Create` saat menambahkan transaksi pgx eksplisit di `GeneratePayroll`.
- **Severity:** Medium — tidak berdampak ke runtime/production, tapi memblokir seluruh test suite unit (build failed, 0 test bisa jalan).

### Langkah Reproduksi
```go
go test ./tests/unit/... -v
```
(dijalankan setelah `PayrollRepository.Create` diubah untuk menerima parameter `tx pgx.Tx`, sebagai bagian dari pembungkusan `GeneratePayroll` dalam transaksi pgx eksplisit — BeginTx → Create → Commit/Rollback)

### Before
```log
tests\unit\payroll_service_test.go:205:65: cannot use payrollRepo (variable of type *mockPayrollRepo)
as repository.PayrollRepository value in argument to service.NewPayrollService: *mockPayrollRepo does
not implement repository.PayrollRepository (wrong type for method Create)
have Create(context.Context, *model.Payroll) error
want Create(context.Context, pgx.Tx, *model.Payroll) error
FAIL github.com/seymourrisey/sistem-penggajian/tests/unit [build failed]
```

### Root Cause
`mockPayrollRepo` di test file meng-implement signature lama `Create(ctx, p)`. Perubahan interface `PayrollRepository` (menambah `BeginTx`, mengubah `Create` untuk menerima `tx pgx.Tx`) adalah breaking change yang tidak otomatis terpropagasi ke mock — Go memaksa mock satisfy interface baru secara eksplisit, jadi test package gagal compile total sebelum satu pun test case sempat jalan.

### Fix (After)
- Tambah `mockTx` (embed `pgx.Tx` nil, override `Commit`/`Rollback`) untuk mensimulasikan transaksi.
- Update `mockPayrollRepo`: implement `BeginTx` (return `mockTx` baru), ubah `Create` untuk menerima parameter `tx pgx.Tx`.
- **Tambahan di luar sekadar fix compile**: masukkan field `wantCommitCalled`/`wantRollbackCalled` di tiap test case untuk verifikasi eksplisit bahwa `Commit()` dipanggil saat sukses dan `Rollback()` dipanggil saat `Create` gagal — sebelumnya tidak ada test yang membuktikan perilaku transaksi ini sama sekali.

### Verifikasi
```go
go test ./tests/unit/... -v
```
```log
=== RUN   TestGeneratePayroll
=== RUN   TestGeneratePayroll/normal_case:_campuran_flat_&_persen,_tunjangan_&_potongan
=== RUN   TestGeneratePayroll/gaji_pokok_nol:_persen_ikut_jadi_nol,_flat_tetap_jalan
=== RUN   TestGeneratePayroll/komponen_kosong:_gaji_bersih_=_gaji_pokok_apa_adanya
=== RUN   TestGeneratePayroll/is_persen_true_murni:_hanya_tunjangan_persen_(10%)
=== RUN   TestGeneratePayroll/is_persen_true_dengan_nominal_0%:_kontribusi_harus_nol,_bukan_galat
=== RUN   TestGeneratePayroll/karyawan_tidak_ditemukan:_harus_short-circuit,_Create_tidak_dipanggil
=== RUN   TestGeneratePayroll/komponenRepo_gagal:_harus_short-circuit,_Create_tidak_dipanggil
=== RUN   TestGeneratePayroll/payroll_sudah_pernah_digenerate_untuk_periode_ini
--- PASS: TestGeneratePayroll (0.00s)
    --- PASS: TestGeneratePayroll/normal_case:_campuran_flat_&_persen,_tunjangan_&_potongan (0.00s)
    --- PASS: TestGeneratePayroll/gaji_pokok_nol:_persen_ikut_jadi_nol,_flat_tetap_jalan (0.00s)
    --- PASS: TestGeneratePayroll/komponen_kosong:_gaji_bersih_=_gaji_pokok_apa_adanya (0.00s)
    --- PASS: TestGeneratePayroll/is_persen_true_murni:_hanya_tunjangan_persen_(10%) (0.00s)
    --- PASS: TestGeneratePayroll/is_persen_true_dengan_nominal_0%:_kontribusi_harus_nol,_bukan_galat (0.00s)
    --- PASS: TestGeneratePayroll/karyawan_tidak_ditemukan:_harus_short-circuit,_Create_tidak_dipanggil (0.00s)
    --- PASS: TestGeneratePayroll/komponenRepo_gagal:_harus_short-circuit,_Create_tidak_dipanggil (0.00s)
    --- PASS: TestGeneratePayroll/payroll_sudah_pernah_digenerate_untuk_periode_ini (0.00s)
PASS
ok  	github.com/seymourrisey/sistem-penggajian/tests/unit
```

---

## Bug #12 — TRUNCATE ... RESTART IDENTITY Gagal karena Privilege Ownership Sequence

- **Endpoint terdampak:** Tidak ada endpoint HTTP langsung — ini kegagalan setup di `TestMain` (`tests/integration/karyawan_api_test.go`), dipicu oleh statement `TRUNCATE ... RESTART IDENTITY CASCADE` yang dijalankan sebagai user `payroll_test_app` untuk membersihkan state database sebelum tiap integration test run.
- **Severity:** Medium — tidak berdampak ke runtime/production, tapi memblokir seluruh integration test suite (TestMain gagal di tahap setup sebelum satu pun test case sempat jalan).

### Langkah Reproduksi

```go
go test ./tests/integration/... -v
```

(dijalankan setelah `TestMain` menambahkan statement `TRUNCATE departemen, karyawan, komponen_gaji, payroll RESTART IDENTITY CASCADE` menggunakan koneksi `payroll_test_app`, sebagai bagian dari strategi reset state test database — opsi truncate-sekali-di-awal yang dipilih untuk menjaga predictability antar test run)

### Before

```log
integration test: terkoneksi ke payroll_test_db, mulai menjalankan test...
2026/07/22 19:57:26 integration test: gagal truncate tabel sebelum test run: ERROR: must be owner of sequence departemen_id_seq (SQLSTATE 42501)
FAIL	github.com/seymourrisey/sistem-penggajian/tests/integration	1.831s
FAIL
```

### Root Cause

`RESTART IDENTITY` pada `TRUNCATE` mensyaratkan role yang menjalankannya memiliki **ownership** atas sequence terkait (`departemen_id_seq`, dst), bukan sekadar privilege DML biasa (`SELECT`/`INSERT`/`UPDATE`/`DELETE`/`TRUNCATE` yang sudah di-`GRANT` ke `payroll_test_app`). Sequence-sequence ini masih dimiliki role yang menjalankan migration (superuser), bukan `payroll_test_app`. PostgreSQL tidak menyediakan level `GRANT` granular untuk "boleh restart sequence" tanpa ownership penuh — satu-satunya fix langsung adalah `ALTER SEQUENCE ... OWNER TO payroll_test_app`, tapi itu memberi `payroll_test_app` hak `ALTER`/`DROP` atas sequence, privilege lebih tinggi dari yang dibutuhkan user aplikasi test (bertentangan dengan prinsip least privilege / NF5).

### Fix (After)

- Hilangkan `RESTART IDENTITY` dari statement truncate — cukup `TRUNCATE departemen, karyawan, komponen_gaji, payroll CASCADE`, tidak menyentuh sequence sama sekali sehingga tidak butuh ownership tambahan.
- Konsekuensi: nilai `id` hasil `INSERT` tidak lagi predictable (1, 2, ...) karena sequence terus naik antar test run. Sebagai gantinya, seluruh `INSERT` di test (termasuk seed dependency `departemen` di `TestMain`) **wajib** mengambil id lewat `RETURNING id`, disimpan ke variabel (mis. `seedDepartemenITID`), bukan pernah di-hardcode.
- Privilege `payroll_test_app` tetap terbatas pada DML + TRUNCATE tabel, tanpa ownership sequence apa pun — konsisten dengan setup privilege user test yang dibuat mirip `payroll_app` production.

### Verifikasi

```go
go test ./tests/integration/... -v
```

```log
integration test: terkoneksi ke payroll_test_db, mulai menjalankan test...
testing: warning: no tests to run
PASS
ok  	github.com/seymourrisey/sistem-penggajian/tests/integration	1.914s [no tests to run]
```

---

## Bug #13 — Response PUT /api/karyawan/:id Mengembalikan `status` dan `created_at` Kosong
- **Endpoint terdampak:** `PUT /api/karyawan/:id` (`internal/handler/karyawan_handler.go`, fungsi `Update`).
- **Severity:** Medium — tidak merusak data di database (kolom `status`/`created_at` di database tetap benar), tapi response API memberi informasi salah ke client (`status` dan `created_at` selalu zero value: `""` dan `0001-01-01T00:00:00Z`).
### Langkah Reproduksi
```go
go test ./tests/integration/... -v -run TestUpdateKaryawan/status_tidak_berubah_meski_dikirim
```
(subtest baru yang sengaja mengirim `"status": "nonaktif"` di body PUT untuk memverifikasi business rule immutability status, lihat `project-design.md` section 2.2)
### Before
```log
karyawan_update_test.go:159: status seharusnya tetap aktif meski dikirim nonaktif di body PUT, got
--- FAIL: TestUpdateKaryawan/status_tidak_berubah_meski_dikirim (0.00s)
```
### Root Cause
Handler `Update` membangun struct `model.Karyawan` hanya dari field request (`req.NIP`, `req.Nama`, dst) sebelum memanggil `svc.Update`. Field `Status` dan `CreatedAt` tidak pernah diisi ke struct ini. Query `UPDATE` di repository sebelumnya hanya `RETURNING updated_at` (di-scan ke `k.UpdatedAt`), sehingga `k.Status` dan `k.CreatedAt` tetap zero value saat `newKaryawanResponse(k)` dipanggil di akhir handler. Bug ini murni di layer response — bukan business rule immutability status yang salah (query `UPDATE` memang sudah benar, tidak menyentuh kolom `status` sama sekali), tapi response yang dibangun dari struct request lokal alih-alih dari data aktual hasil update di database.
### Fix (After)
Ubah `RETURNING` di `karyawan_repository.go` fungsi `Update` untuk menyertakan `status` dan `created_at`, di-scan langsung ke `k.Status` dan `k.CreatedAt`:
```go
RETURNING status, created_at, updated_at
```
```go
.Scan(&k.Status, &k.CreatedAt, &k.UpdatedAt)
```
Tidak ada query tambahan (masih 1 round-trip), tidak ada perubahan di `service`/`handler`.
### Verifikasi
```go
go test ./tests/integration/... -v -run TestUpdateKaryawan
```
```log
--- PASS: TestUpdateKaryawan (0.01s)
    --- PASS: TestUpdateKaryawan/sukses_200 (0.01s)
    --- PASS: TestUpdateKaryawan/tidak_ditemukan_404 (0.00s)
    --- PASS: TestUpdateKaryawan/validasi_gagal_400_nip_kosong (0.00s)
    --- PASS: TestUpdateKaryawan/status_tidak_berubah_meski_dikirim (0.00s)
PASS
```

---

## Verifikasi Tambahan Menggunakan Delve (Go Debugger)

Seluruh bug pada dokumen ini ditemukan melalui exploratory testing manual menggunakan Postman. Setelah akar masalah diidentifikasi dan didokumentasikan, dilakukan verifikasi tambahan menggunakan Delve (Go debugger) untuk menelusuri alur eksekusi program dan mengonfirmasi root cause pada beberapa kasus yang representatif. Root cause kedua kasus di bawah **sudah diketahui dan sudah di-fix** sebelumnya (lihat Bug #8 dan #13 di atas); untuk keperluan verifikasi ini, fix pada kode sementara di-revert ke versi buggy, direproduksi ulang, ditelusuri lewat Delve, lalu dikembalikan ke versi fixed.

### Tujuan Penggunaan Delve

Penggunaan Delve pada bagian ini bertujuan untuk:
- memasang breakpoint pada modul yang dianalisis;
- menginspeksi nilai variabel saat runtime;
- menelusuri alur eksekusi fungsi;
- mengonfirmasi bahwa root cause yang telah diidentifikasi benar-benar terjadi pada saat program berjalan.

### Verifikasi Bug #8 (IDOR) via Delve

**Setup:** query `UPDATE` di `komponenGajiRepository.Update` sementara dikembalikan ke versi sebelum fix — klausa `AND karyawan_id = $6` dihapus dari query **dan** argumen `k.KaryawanID` dihapus dari pemanggilan `Exec` (supaya jumlah placeholder dan argumen tetap cocok). Untuk keperluan verifikasi debugger, perubahan kode sementara dikembalikan ke kondisi sebelum perbaikan (revert lokal tanpa commit), kemudian setelah proses debugging selesai dikembalikan lagi ke versi final — tidak ada perubahan histori Git.

**Breakpoint:**
```
(dlv) break internal/repository.(*komponenGajiRepository).Update
Breakpoint 1 set at ... komponen_gaji_repository.go:127
(dlv) break internal/repository/komponen_gaji_repository.go:144
Breakpoint 2 set at ... komponen_gaji_repository.go:144
(dlv) continue
```

**Request pemicu (reproduksi langkah asli Bug #8):**
```
PUT /api/karyawan/1/komponen-gaji/15
```
(`komponen_id=15` sebenarnya milik `karyawan_id=2`, bukan `karyawan_id=1` yang ada di URL)

**Inspect variable saat breakpoint #1 (baris 127, awal fungsi):**
```
(dlv) print k.ID
15
(dlv) print k.KaryawanID
1
```
Konfirmasi: parameter yang diterima fungsi persis sesuai skenario bug — `karyawan_id` dari URL (1) tidak sama dengan pemilik asli komponen (2).

**Inspect variable saat breakpoint #2 (baris 144, setelah `Exec` selesai):**
```
(dlv) print tag
github.com/jackc/pgx/v5/pgconn.CommandTag {
	s: "UPDATE 1",
}
(dlv) call tag.RowsAffected()
Values returned:
	~r0: 1
(dlv) print err
error nil
```
`RowsAffected()` dipanggil dari Delve untuk memverifikasi jumlah row yang benar-benar berubah di database.

Konfirmasi langsung dari eksekusi nyata: `err` = nil, `tag.RowsAffected()` = **1** — satu row benar-benar berubah di database meskipun `karyawan_id` di URL tidak cocok dengan pemilik asli. Ini membuktikan root cause Bug #8 secara langsung (bukan hanya lewat pembacaan kode statis): query `WHERE id = $5` tanpa klausa `AND karyawan_id` tidak pernah memvalidasi kepemilikan, sehingga request berhasil (`200 OK`, dikonfirmasi via log server `[GIN] ... | 200 | ... | PUT "/api/karyawan/1/komponen-gaji/15"`) dan mengubah data milik karyawan lain.

**Setelah verifikasi:** kode query & argumen `Exec` dikembalikan ke versi fixed (`AND karyawan_id = $6` + `k.KaryawanID` di `Exec`), sesuai fix yang sudah tercatat di Bug #8.

### Verifikasi Bug #13 (Response Kosong) via Delve

**Setup:** fungsi `Update` di `karyawan_repository.go` sementara dikembalikan ke versi sebelum fix — `RETURNING status, created_at, updated_at` diganti balik jadi `RETURNING updated_at`, dan `Scan(&k.Status, &k.CreatedAt, &k.UpdatedAt)` diganti balik jadi `Scan(&k.UpdatedAt)`. Sama seperti verifikasi Bug #8, ini revert lokal tanpa commit — dikembalikan lagi ke versi final setelah debugging selesai.

**Breakpoint (di handler, bukan repository — bug ini soal struct yang dipakai buat build response):**
```
(dlv) break internal/handler.(*KaryawanHandler).Update
Breakpoint 1 set at ... karyawan_handler.go:173
(dlv) continue
```

**Request pemicu (reproduksi langkah asli Bug #13):**
```
PUT /api/karyawan/1
```
Body Request menggunakan payload yang sama seperti reproduksi Bug #13.

**Cari titik tepat sebelum response dikirim:**
```
(dlv) list internal/handler/karyawan_handler.go:202
```
Menunjukkan `c.JSON(http.StatusOK, newKaryawanResponse(k))` di baris 207 — breakpoint kedua ditaruh di situ, tepat setelah `h.svc.Update(...)` selesai dan sebelum struct `k` dikirim ke client.
```
(dlv) break internal/handler/karyawan_handler.go:207
(dlv) continue
```

**Inspect variable saat breakpoint #2 (baris 207):**
```
(dlv) print k.Status
""
(dlv) print k.CreatedAt
time.Time(0001-01-01T00:00:00Z){
	wall: 0,
	ext: 0,
	loc: *time.Location nil,
}
(dlv) print k.UpdatedAt
time.Time(2026-07-25T08:40:02Z){
	wall: 214279000,
	ext: 63920565602,
	loc: *time.Location nil,
}
```
Konfirmasi langsung dari eksekusi nyata: `k.Status` dan `k.CreatedAt` tetap zero value (`""` dan `0001-01-01T00:00:00Z`), sementara `k.UpdatedAt` terisi normal (`2026-07-25T08:40:02Z`) — kontras ini membuktikan root cause Bug #13 secara langsung: hanya `updated_at` yang di-scan dari hasil `RETURNING`, sehingga `Status` dan `CreatedAt` pada struct `k` tidak pernah diisi ulang dari database dan tetap zero value Go saat `newKaryawanResponse(k)` dipanggil.

**Setelah verifikasi:** kode `RETURNING`/`Scan` di `karyawan_repository.go` dikembalikan ke versi fixed (`RETURNING status, created_at, updated_at` + `Scan(&k.Status, &k.CreatedAt, &k.UpdatedAt)`), sesuai fix yang sudah tercatat di Bug #13.

### Kesimpulan

Melalui Delve, proses debugging tidak hanya dilakukan melalui inspeksi source code, tetapi juga melalui observasi langsung terhadap nilai variabel dan hasil eksekusi program saat runtime. Hasil observasi tersebut konsisten dengan root cause yang telah didokumentasikan pada Bug #8 dan Bug #13, sehingga memperkuat validitas analisis dan perbaikan yang dilakukan.

---


## Ringkasan

| # | Bug | Severity | Status |
|---|---|---|---|
| 1 | FK violation delete departemen → 500 | Medium | Fixed & Verified |
| 2 | Raw validator error (field kosong) | Medium | Fixed & Verified |
| 3 | Raw decimal parse error | Medium | Fixed & Verified |
| 4 | Raw JSON type mismatch error | Medium | Fixed & Verified |
| 5 | Raw validator error (partial update) | Medium | Fixed & Verified |
| 6 | Duplikat komponen gaji tersimpan | High | Fixed & Verified |
| 7 | Error duplikat saat update bocor mentah | Medium | Fixed & Verified |
| 8 | IDOR — update komponen gaji lintas karyawan | High | Fixed & Verified |
| 9 | Double response write pada PUT departemen | Medium | Fixed & Verified |
| 10 | Response JSON Tanggal Menggunakan RFC3339, Tidak Sesuai API Contract | Medium | Fixed & Verified |
| 11 | Mock PayrollRepository Tidak Sinkron Setelah Interface Berubah (Transaksi pgx) | Medium | Fixed & Verified |
| 12 | TRUNCATE ... RESTART IDENTITY Gagal karena Privilege Ownership Sequence | Medium | Fixed & Verified |
| 13 | Response PUT /api/karyawan/:id Mengembalikan `status` dan `created_at` Kosong | Medium | Fixed & Verified |

Seluruh kasus ditemukan melalui exploratory testing manual (Postman), bukan simulasi/hipotesis, setiap "Before" adalah response aktual yang tercatat, dan setiap "Verifikasi" adalah hasil retest aktual setelah fix diterapkan.
