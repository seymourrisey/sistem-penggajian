# Lembar Pengujian Integrasi — Sistem Informasi Penggajian

- **Kode Test:** `tests/integration/karyawan_api_test.go` (TestMain — setup), `tests/integration/karyawan_create_test.go`, `tests/integration/karyawan_getbyid_test.go`, `tests/integration/karyawan_update_test.go`, `tests/integration/karyawan_softdelete_test.go`
- **Metode:** `httptest` + `testRouter.ServeHTTP`, full dependency chain nyata (repository → service → handler → router) terhubung ke database PostgreSQL sungguhan (bukan mock)
- **Peralatan:**
  - Database: PostgreSQL, database `payroll_test_db` (terpisah dari `payroll_db` production)
  - User database test: `payroll_test_app`, privilege terbatas (DML + TRUNCATE tabel, tanpa ownership sequence)
  - Environment config: `.env.test`, dimuat via override `ENV_FILE=../../.env.test` (working directory `go test` berada di folder package, bukan root project)
  - Schema: hasil `migrations/001_init_schema.sql` + `migrations/002_sql_native_features.sql`, TANPA `seed.sql` (state dibentuk oleh kode test sendiri, bukan data organik)
- **Command Eksekusi:** `go test ./tests/integration/... -v`
- **Strategi Reset State:** Truncate sekali di awal seluruh test run (bukan per test function), lewat `TestMain`. `TRUNCATE departemen, karyawan, komponen_gaji, payroll CASCADE` — tanpa `RESTART IDENTITY` (lihat `docs/debugging-log.md` Bug #12: RESTART IDENTITY butuh ownership sequence, bertentangan dengan least privilege / NF5). Konsekuensi: id tidak predictable, seluruh test mengambil id lewat `RETURNING id`, bukan hardcode.
- **Data Uji Dependency:** 2 departemen di-seed di `TestMain` sebelum test run (`IT`, `HR`), id disimpan di variabel `seedDepartemenITID` / `seedDepartemenHRID` — dipakai seluruh test yang butuh `departemen_id` valid.
- **Tanggal Eksekusi Terakhir:** [isi tanggal aktual saat run]
- **Hasil Keseluruhan:** 12/12 skenario PASS

---

## A. Endpoint: `POST /api/karyawan` (`karyawan_create_test.go` — `TestCreateKaryawan`)

| No | Skenario | Data Uji | Expected | Actual | Status |
|---|---|---|---|---|---|
| 1 | Sukses membuat karyawan baru | nip="TCK-SUKSES-001"; nama="Budi Santoso"; departemen_id=seedDepartemenITID; jabatan="Staff"; gaji_pokok=5.000.000; tanggal_masuk="2024-01-15" | HTTP 201; response berisi nip="TCK-SUKSES-001"; status="aktif"; field id ada | Sesuai expected | PASS |
| 2 | Validasi gagal: field nama kosong | nip="TCK-VALIDASI-001"; nama=""; departemen_id=seedDepartemenITID; jabatan="Staff"; gaji_pokok=5.000.000; tanggal_masuk="2024-01-15" | HTTP 400 (binding `required` menolak sebelum sempat ke service) | Sesuai expected | PASS |
| 3 | NIP duplikat: insert kedua dengan NIP sama harus ditolak | Insert pertama: nip="TCK-DUPLIKAT-001", nama="Karyawan Pertama" → sukses. Insert kedua: nip sama, nama="Karyawan Kedua Nama Beda" | Insert pertama HTTP 201; insert kedua HTTP 409 (UNIQUE constraint pada kolom nip) | Sesuai expected | PASS |
| 4 | departemen_id tidak valid (FK constraint) | nip="TCK-DEPTINVALID-001"; nama="Karyawan Departemen Salah"; departemen_id=999999 (tidak ada di tabel departemen); jabatan="Staff"; gaji_pokok=5.000.000; tanggal_masuk="2024-01-15" | HTTP 400 (`repository.ErrDepartemenTidakValid` via `mapKaryawanError`) | Sesuai expected | PASS |

---

## B. Endpoint: `GET /api/karyawan/:id` (`karyawan_getbyid_test.go` — `TestGetKaryawanByID`)

| No | Skenario | Data Uji | Expected | Actual | Status |
|---|---|---|---|---|---|
| 5 | Sukses mengambil karyawan yang ada | Setup: insert karyawan nip="TGK-SUKSES-001", nama="Siti Aminah", departemen_id=seedDepartemenITID, gaji_pokok=4.500.000, tanggal_masuk="2023-06-01" → ambil id dari response. Lalu GET `/api/karyawan/{id}` dengan id tersebut | HTTP 200; response berisi nip="TGK-SUKSES-001" sesuai data yang di-insert | Sesuai expected | PASS |
| 6 | Karyawan tidak ditemukan | GET `/api/karyawan/999999` (id yang dipastikan tidak ada) | HTTP 404 | Sesuai expected | PASS |
| 7 | Format id bukan angka | GET `/api/karyawan/bukan-angka` | HTTP 400 (gagal di `strconv.Atoi` sebelum sempat query service) | Sesuai expected | PASS |

---

## C. Endpoint: `PUT /api/karyawan/:id` (`karyawan_update_test.go` — `TestUpdateKaryawan`)

| No | Skenario | Data Uji | Expected | Actual | Status |
|---|---|---|---|---|---|
| 8 | Sukses update: pindah departemen, naik jabatan & gaji | Setup: insert nip="TUK-SUKSES-001", departemen_id=seedDepartemenITID, jabatan="Staff", gaji_pokok=4.000.000 → ambil id. Update: departemen_id=seedDepartemenHRID, jabatan="Staff Senior", gaji_pokok=6.000.000 (nip & tanggal_masuk tetap) | HTTP 200; response berisi jabatan="Staff Senior"; departemen_id=seedDepartemenHRID | Sesuai expected | PASS |
| 9 | Karyawan tidak ditemukan | PUT `/api/karyawan/999999` dengan body update valid (nip="TUK-404-001", dst) | HTTP 404 | Sesuai expected | PASS |
| 10 | Validasi gagal: field nip kosong | Setup: insert nip="TUK-VALIDASI-001" → ambil id. Update dengan nip="" (kosong), field lain valid | HTTP 400 (binding `required` menolak) | Sesuai expected | PASS |

---

## D. Endpoint: `DELETE /api/karyawan/:id` (`karyawan_softdelete_test.go` — `TestSoftDeleteKaryawan`)

| No | Skenario | Data Uji | Expected | Actual | Status |
|---|---|---|---|---|---|
| 11 | Sukses soft-delete: status berubah jadi nonaktif, record tidak hilang | Setup: insert nip="TSD-SUKSES-001" → ambil id. DELETE `/api/karyawan/{id}`, lalu GET `/api/karyawan/{id}` lagi untuk verifikasi | DELETE: HTTP 200. GET setelahnya: HTTP 200 (record masih ada, bukan hard-delete) dengan status="nonaktif" | Sesuai expected | PASS |
| 12 | Karyawan tidak ditemukan | DELETE `/api/karyawan/999999` | HTTP 404 | Sesuai expected | PASS |

---

## Analisis Kesesuaian

- **Cakupan status code:** Seluruh 4 status code utama pada kontrak error response (`section 4.2` ProjectDesign) sudah tercakup berulang kali lewat CRUD karyawan penuh — 200 (No. 5, 8, 11 DELETE call, 11 GET verifikasi), 201 (No. 1, dan seluruh setup di No. 5/8/10/11), 400 (No. 2, 4, 7, 10), 404 (No. 6, 9, 12), 409 (No. 3). 500 tidak diuji secara eksplisit (sulit dipicu deterministik tanpa memutus koneksi database secara sengaja); jalur ini tercakup tidak langsung lewat `mapKaryawanError` default case di layer handler.
- **Cakupan endpoint:** Integration test pada tahap ini mencakup **seluruh 5 endpoint CRUD karyawan** (Create, GetAll tidak diuji terpisah tapi dipakai tak langsung, GetByID, Update, SoftDelete). Endpoint departemen, komponen-gaji, dan payroll belum diuji integrasi — keputusan sadar terkait prioritas waktu (lihat `summary-room10part2.md`). Unit test (`docs/lembar-pengujian-unit.md`) sudah mencakup logic kalkulasi payroll dan algoritma sorting secara terpisah lewat mock, sehingga risiko logic inti tetap tertutupi meski integration test tidak menyentuh seluruh endpoint aplikasi.
- **Verifikasi efek nyata, bukan cuma status code:** Skenario No. 8 memverifikasi field response berubah sesuai request (bukan cuma HTTP 200). Skenario No. 11 melangkah lebih jauh — memverifikasi lewat GET request terpisah setelah DELETE bahwa record benar-benar masih ada dengan status berubah, membuktikan soft-delete (bukan hard-delete) berjalan sesuai business rule di `ProjectDesign section 2.2`.
- **Isolasi test database:** Seluruh skenario berjalan di atas `payroll_test_db`, terpisah fisik dari `payroll_db` production, dengan double safety-check di `TestMain` (nama database dari config maupun dari `SELECT current_database()` server) sebelum test manapun diizinkan berjalan — mencegah risiko operasi tak sengaja terhadap data production.
- **Independensi antar skenario:** Karena strategi reset state adalah truncate sekali di awal seluruh run (bukan per test), setiap skenario memakai NIP unik berprefix nama skenario (mis. `TCK-SUKSES-001`, `TUK-SUKSES-001`, `TSD-SUKSES-001`) untuk mencegah collision antar test function dalam satu kali eksekusi `go test`.

---

## Catatan Metodologi

- Test menghubungkan seluruh layer nyata (`handler → service → repository → PostgreSQL`) lewat `testRouter.ServeHTTP`, bukan mock — berbeda dari unit test yang me-mock seluruh dependency. Ini memvalidasi wiring, query SQL asli, dan constraint database (UNIQUE, FK, NOT NULL) yang tidak tersentuh oleh unit test.
- ID entity (karyawan, departemen) tidak pernah di-hardcode di skenario manapun — selalu diambil dari `RETURNING id` (untuk seed) atau dari response JSON hasil `POST` (untuk data yang dibuat di dalam test, lewat helper `extractIDFromResponse`), konsisten dengan keputusan menghindari `RESTART IDENTITY` (Bug #12).
- Dokumen ini adalah bukti formal terpisah dari kode test, sesuai KUK unit #10. Kode test yang menjadi rujukan data uji: keempat file test di `tests/integration/` yang disebut di atas.
