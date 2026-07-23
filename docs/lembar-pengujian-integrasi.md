# Lembar Pengujian Integrasi — Sistem Informasi Penggajian

- **Kode Test:** `tests/integration/karyawan_api_test.go` (TestMain — setup), `karyawan_create_test.go`, `karyawan_getbyid_test.go`, `karyawan_update_test.go`, `karyawan_softdelete_test.go`, `payroll_generate_test.go`, `payroll_riwayat_test.go`, `komponen_gaji_list_test.go`, `departemen_delete_test.go`
- **Metode:** `httptest` + `testRouter.ServeHTTP`, full dependency chain nyata (repository → service → handler → router) terhubung ke database PostgreSQL sungguhan (bukan mock)
- **Peralatan:**
  - Database: PostgreSQL, database `payroll_test_db` (terpisah dari `payroll_db` production)
  - User database test: `payroll_test_app`, privilege terbatas (DML + TRUNCATE tabel, tanpa ownership sequence)
  - Environment config: `.env.test`, dimuat via override `ENV_FILE=../../.env.test` (working directory `go test` berada di folder package, bukan root project)
  - Schema: hasil `migrations/001_init_schema.sql` + `migrations/002_sql_native_features.sql`, TANPA `seed.sql` (state dibentuk oleh kode test sendiri, bukan data organik)
- **Command Eksekusi:** `go test ./tests/integration/... -v`
- **Strategi Reset State:** Truncate sekali di awal seluruh test run (bukan per test function), lewat `TestMain`. `TRUNCATE departemen, karyawan, komponen_gaji, payroll CASCADE` — tanpa `RESTART IDENTITY` (lihat `docs/debugging-log.md` Bug #12: RESTART IDENTITY butuh ownership sequence, bertentangan dengan least privilege / NF5). Konsekuensi: id tidak predictable, seluruh test mengambil id lewat `RETURNING id`, bukan hardcode.
- **Data Uji Dependency:** 2 departemen di-seed di `TestMain` sebelum test run (`IT`, `HR`), id disimpan di variabel `seedDepartemenITID` / `seedDepartemenHRID` — dipakai seluruh test yang butuh `departemen_id` valid. Skenario yang menguji business rule departemen (section H) sengaja membuat departemen baru sendiri, bukan memakai seed ini, supaya tidak merusak data yang dipakai test lain dalam test binary yang sama.
- **Tanggal Eksekusi Terakhir:** 23 July 2026
- **Hasil Keseluruhan:** 23/23 skenario PASS

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
| 11 | Business rule: status tidak berubah meski dikirim di body PUT | Setup: insert nip="TUK-STATUS-001", status default "aktif" → ambil id. Update dengan body menyertakan `"status": "nonaktif"` (field asing, bersamaan dengan jabatan="Staff Senior" yang valid) | HTTP 200; response `status` tetap "aktif" (bukan "nonaktif"); `jabatan` tetap ter-update ke "Staff Senior"; verifikasi GET ulang ke database juga `status`="aktif" | Sesuai expected (setelah fix Bug #13, lihat catatan di bawah) | PASS |

---

## D. Endpoint: `DELETE /api/karyawan/:id` (`karyawan_softdelete_test.go` — `TestSoftDeleteKaryawan`)

| No | Skenario | Data Uji | Expected | Actual | Status |
|---|---|---|---|---|---|
| 12 | Sukses soft-delete: status berubah jadi nonaktif, record tidak hilang | Setup: insert nip="TSD-SUKSES-001" → ambil id. DELETE `/api/karyawan/{id}`, lalu GET `/api/karyawan/{id}` lagi untuk verifikasi | DELETE: HTTP 200. GET setelahnya: HTTP 200 (record masih ada, bukan hard-delete) dengan status="nonaktif" | Sesuai expected | PASS |
| 13 | Karyawan tidak ditemukan | DELETE `/api/karyawan/999999` | HTTP 404 | Sesuai expected | PASS |

---

## E. Endpoint: `POST /api/payroll/generate` (`payroll_generate_test.go` — `TestGeneratePayroll`)

| No | Skenario | Data Uji | Expected | Actual | Status |
|---|---|---|---|---|---|
| 14 | Sukses generate, kalkulasi gaji_bersih end-to-end benar | Karyawan nip="TPG-SUKSES-001", gaji_pokok=5.000.000. Komponen: tunjangan tetap "Tunjangan Transport" nominal=500.000 (is_persen=false); potongan persen "BPJS Kesehatan" nominal=2 (is_persen=true, basis gaji_pokok flat). Generate periode="2026-08-01" | HTTP 201; total_tunjangan=500.000; total_potongan=100.000 (5.000.000×2%); gaji_bersih=5.400.000 (5.000.000+500.000−100.000); status="draft" | Sesuai expected | PASS |
| 15 | Duplikat karyawan_id+periode ditolak | Karyawan nip="TPG-DUPLIKAT-001". Generate periode="2026-08-01" dua kali dengan karyawan_id sama | Generate pertama HTTP 201; generate kedua HTTP 409 (UNIQUE constraint karyawan_id+periode) | Sesuai expected | PASS |

---

## F. Endpoint: `GET /api/payroll/:karyawan_id/riwayat` (`payroll_riwayat_test.go` — `TestGetRiwayatPayroll`)

| No | Skenario | Data Uji | Expected | Actual | Status |
|---|---|---|---|---|---|
| 16 | karyawan_id tidak ditemukan | GET `/api/payroll/999999/riwayat` | HTTP 404 | Sesuai expected | PASS |
| 17 | Karyawan ada, belum punya riwayat payroll | Karyawan baru nip="TPR-KOSONG-001", belum pernah generate payroll. GET riwayat | HTTP 200; response array kosong `[]` | Sesuai expected | PASS |
| 18 | Karyawan dengan riwayat payroll aktual | Karyawan nip="TPR-SUKSES-001", generate 1 payroll periode="2026-09-01". GET riwayat | HTTP 200; response 1 item dengan nip="TPR-SUKSES-001", periode="2026-09-01", karyawan_id sesuai | Sesuai expected | PASS |

---

## G. Endpoint: `GET /api/karyawan/:id/komponen-gaji` (`komponen_gaji_list_test.go` — `TestGetKomponenGajiByKaryawanID`)

| No | Skenario | Data Uji | Expected | Actual | Status |
|---|---|---|---|---|---|
| 19 | Hasil terurut berdasarkan nominal terbesar (sorting manual F7) | Karyawan nip="TKG-SORT-001". 4 komponen di-insert TIDAK berurutan: 250.000, 500.000, 100.000, 350.000. GET list komponen | HTTP 200; urutan response persis descending: 500.000, 350.000, 250.000, 100.000 — membuktikan `util.SortKomponenGajiByNominalDesc` diterapkan, bukan urutan insert | Sesuai expected | PASS |
| 20 | Karyawan belum punya komponen gaji | Karyawan baru nip="TKG-KOSONG-001", belum ada komponen gaji | HTTP 200; response array kosong `[]` | Sesuai expected | PASS |

---

## H. Endpoint: `DELETE /api/departemen/:id` (`departemen_delete_test.go` — `TestDeleteDepartemen`)

| No | Skenario | Data Uji | Expected | Actual | Status |
|---|---|---|---|---|---|
| 21 | Masih direferensikan karyawan aktif, ditolak (FK constraint) | Departemen baru "Departemen Uji Delete FK". Karyawan nip="TDD-FK-001" dibuat mereferensikan departemen tsb. DELETE departemen | HTTP 409 (`repository.ErrDepartemenMasihDipakai`, mapping dari PG error code 23503); verifikasi GET departemen setelahnya tetap HTTP 200 (tidak terhapus) | Sesuai expected | PASS |
| 22 | Tidak direferensikan, sukses hard-delete | Departemen baru "Departemen Uji Delete Sukses", tidak pernah dipakai karyawan manapun. DELETE departemen | HTTP 200; verifikasi GET departemen setelahnya HTTP 404 (benar-benar terhapus, bukan soft-delete) | Sesuai expected | PASS |
| 23 | Departemen tidak ditemukan | DELETE `/api/departemen/999999` | HTTP 404 | Sesuai expected | PASS |

---

## Analisis Kesesuaian

- **Cakupan status code:** Seluruh 4 status code utama pada kontrak error response (`section 4.2` ProjectDesign) sudah tercakup berulang kali lintas 5 endpoint berbeda — 200 (No. 5, 8, 11, 12 DELETE call, 16-18, 19-20, 22), 201 (No. 1, 14, 15, dan seluruh setup entity di berbagai skenario), 400 (No. 2, 4, 7, 10), 404 (No. 6, 9, 13, 16, 23), 409 (No. 3, 15, 21). 500 tidak diuji secara eksplisit (sulit dipicu deterministik tanpa memutus koneksi database secara sengaja); jalur ini tercakup tidak langsung lewat `mapKaryawanError`/`mapPayrollError`/`mapDepartemenError`/`mapKomponenGajiError` default case di masing-masing handler.
- **Cakupan endpoint:** Integration test sekarang mencakup **5 endpoint karyawan CRUD lengkap**, **generate & riwayat payroll**, **list komponen gaji (termasuk sorting manual)**, dan **delete departemen dengan FK constraint** — total 8 file test menutupi seluruh 9 skenario wajib yang disepakati di awal Room 10. Laporan agregat payroll (`GET /api/payroll/laporan`) dan CRUD departemen selain delete (create/update/getall/getbyid) belum diuji integrasi — di luar scope 9 skenario wajib, tidak ditambahkan untuk menjaga fokus waktu, tapi logic-nya sudah tercakup lewat unit test dan pengujian manual Postman selama development.
- **Verifikasi efek nyata, bukan cuma status code:** No. 8 memverifikasi field response berubah sesuai request. No. 12 memverifikasi lewat GET terpisah bahwa soft-delete (bukan hard-delete) benar terjadi. **No. 11** memverifikasi business rule immutability `status` lewat 2 lapis — response PUT dan GET ulang ke database — dan menemukan bug nyata (Bug #13) dalam prosesnya, bukan cuma menguji jalur yang sudah pasti benar. **No. 14** memverifikasi kalkulasi numerik end-to-end (bukan cuma status code) dengan angka yang dihitung manual dan dibandingkan presisi memakai `decimal.Decimal`. **No. 19** memverifikasi urutan elemen array, bukan cuma keberadaan data. **No. 21-22** memverifikasi konsekuensi nyata di database (departemen tetap ada / benar hilang) lewat GET susulan, bukan hanya percaya status code DELETE.
- **Isolasi test database:** Seluruh skenario berjalan di atas `payroll_test_db`, terpisah fisik dari `payroll_db` production, dengan double safety-check di `TestMain` (nama database dari config maupun dari `SELECT current_database()` server) sebelum test manapun diizinkan berjalan.
- **Independensi antar skenario:** Karena strategi reset state adalah truncate sekali di awal seluruh run, setiap skenario memakai NIP/nama-departemen unik berprefix nama skenario (mis. `TCK-SUKSES-001`, `TPG-SUKSES-001`, `TDD-FK-001`, "Departemen Uji Delete FK") untuk mencegah collision antar test function. Periode payroll juga sengaja dibedakan antar file test (`2026-08-01` di section E, `2026-09-01` di section F) untuk menghindari bentrok UNIQUE constraint karyawan_id+periode lintas file.
- **Bug ditemukan lewat integration test (bukti nilai integration test di luar unit test):** Skenario No. 11 menemukan Bug #13 — response `PUT /api/karyawan/:id` mengembalikan `status` dan `created_at` kosong karena handler membangun response dari struct request lokal, bukan data hasil `RETURNING` di repository. Bug ini tidak terdeteksi oleh unit test (yang bekerja dengan mock, bukan database sungguhan) — murni ditemukan karena integration test memverifikasi response API sungguhan lintas seluruh layer. Detail lengkap: `docs/debugging-log.md` Bug #13.

---

## Catatan Metodologi

- Test menghubungkan seluruh layer nyata (`handler → service → repository → PostgreSQL`) lewat `testRouter.ServeHTTP`, bukan mock — berbeda dari unit test yang me-mock seluruh dependency. Ini memvalidasi wiring, query SQL asli, dan constraint database (UNIQUE, FK, NOT NULL) yang tidak tersentuh oleh unit test.
- ID entity (karyawan, departemen) tidak pernah di-hardcode di skenario manapun — selalu diambil dari `RETURNING id` (untuk seed) atau dari response JSON hasil `POST` (untuk data yang dibuat di dalam test, lewat helper `extractIDFromResponse`), konsisten dengan keputusan menghindari `RESTART IDENTITY` (Bug #12).
- Nilai `decimal.Decimal` pada response JSON (`gaji_bersih`, `total_tunjangan`, `total_potongan`, `nominal`) di-serialize sebagai string berkuotasi oleh library `shopspring/decimal`, bukan number JSON mentah — assertion numerik dilakukan lewat parsing `decimal.NewFromString` dan perbandingan `.Equal()`, bukan perbandingan string langsung, supaya tidak salah gagal akibat perbedaan representasi trailing zero (mis. "500000" vs "500000.00").
- Dokumen ini adalah bukti formal terpisah dari kode test, sesuai KUK unit #10. Kode test yang menjadi rujukan data uji: kedelapan file test di `tests/integration/` yang disebut di atas.
