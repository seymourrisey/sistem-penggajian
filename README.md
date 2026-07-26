# Sistem Penggajian

REST API sistem penggajian berbasis web. Mencakup CRUD karyawan/departemen, pengelolaan komponen gaji (tunjangan/potongan), generate slip gaji per periode, riwayat gaji, dan laporan agregat per departemen.

Detail rancangan lengkap (requirements, skema database, justifikasi desain, mapping kompetensi) ada di `docs/project-design.md`.

Frontend web tampilan untuk integrasi pada repository https://github.com/seymourrisey/sistem-penggajian-web

---

## Tech Stack

| Layer | Teknologi |
|---|---|
| Backend | Golang, Gin Gonic (web framework) |
| Database | PostgreSQL, diakses via `pgx` connection pool |
| Nilai uang | `shopspring/decimal` (presisi, hindari floating point error) |
| Testing | `testing` package (unit), `httptest` (integration) |
| Profiling | `pprof` |
| Debugging | Exploratory testing manual (Postman); Delve sebagai verifikasi tambahan pada kasus terpilih |

---

## Prasyarat

- Go 1.21+
- PostgreSQL 14+
- pgAdmin4 (opsional, untuk eksekusi migration manual)

---

## Setup

### 1. Clone & Environment
```bash
git clone https://github.com/seymourrisey/sistem-penggajian.git
cd sistem-penggajian
cp .env.example .env
```
Isi `.env` sesuai koneksi database lokal (host, port, nama DB, user, password).

### 2. Setup Database & User Privilege Terbatas
Aplikasi **tidak** menggunakan user `postgres` (superuser). Buat database dulu, lalu jalankan script setup user dedicated dengan privilege terbatas:
```bash
psql -U postgres -c "CREATE DATABASE payroll_db;"
psql -U postgres -d payroll_db -f migrations/setup-db-user-privilege-terbatas.sql
```
Script ini membuat user `payroll_app` dan meng-grant `SELECT, INSERT, UPDATE, DELETE` pada schema aplikasi — tanpa `DROP`/`CREATE`, sesuai prinsip least privilege (lihat `docs/project-design.md` section 2.4) dengan catatan mohon di edit `setup-db-user-privilege-terbatas.sql` pada bagian ```LOGIN PASSWORD 'ganti_password_kuat_disini'```.

### 3. Jalankan Migration (urutan wajib berurutan)
```bash
psql -U postgres -d payroll_db -f migrations/001_init_schema.sql
psql -U postgres -d payroll_db -f migrations/002_sql_native_features.sql
psql -U postgres -d payroll_db -f migrations/seed.sql
```
- `001_init_schema.sql` — DDL struktural (tabel, index, constraint).
- `002_sql_native_features.sql` — function, trigger, view, stored procedure (bergantung pada skema di `001`, wajib dijalankan setelahnya).
- `seed.sql` — data uji (kombinasi dataset publik + data sintetis, lihat `docs/data-provenance.md` untuk detail provenance).

---

## Menjalankan Aplikasi

```bash
go run cmd/api/main.go
```
Server berjalan di port sesuai `.env` (default `:8080`). Cek server hidup:
```bash
curl http://localhost:8080/api/departemen
```

---

## Menjalankan Test

### Unit Test
```bash
go test ./tests/unit/... -v
```
Mencakup logic kalkulasi gaji (`payroll_service_test.go`) dan algoritma sorting manual (`sort_test.go`). Tidak butuh koneksi database.

### Integration Test
```bash
go test ./tests/integration/... -v
```
Butuh database test terpisah (`payroll_test_db`) dan user `payroll_test_app` dengan privilege setara `payroll_app` production (DML + TRUNCATE tabel, tanpa ownership sequence). Jalankan migration yang sama ke `payroll_test_db` sebelum test pertama kali dijalankan.

Dokumen formal hasil pengujian (skenario, data uji, status pass/fail) ada di `docs/lembar-pengujian-unit.md` dan `docs/lembar-pengujian-integrasi.md`.

---

## Struktur Folder

```
sistem-penggajian/
├── cmd/api/main.go          # entrypoint aplikasi
├── internal/
│   ├── config/               # load environment/config
│   ├── model/                 # struct representasi data
│   ├── repository/            # akses database (query SQL)
│   ├── service/                # business logic & kalkulasi gaji
│   ├── handler/                # HTTP handler (Gin)
│   ├── util/sort.go            # algoritma sorting manual (insertion sort)
│   └── router/router.go        # routing endpoint
├── pkg/database/postgres.go  # koneksi pgx pool, reusable
├── migrations/                # DDL, fitur SQL native, seed data
├── tests/
│   ├── unit/                   # test service & algoritma, tanpa DB
│   └── integration/            # test handler end-to-end via httptest
└── docs/                     # dokumentasi non-kode (lihat section di bawah)
```
Dependency flow: `handler → service → repository → model`. Layer `handler` tidak boleh mengakses `repository`/database secara langsung.

---

## Daftar Endpoint API

| Method | Endpoint | Fungsi |
|---|---|---|
| POST | `/api/departemen` | Tambah departemen |
| GET | `/api/departemen` | List seluruh departemen |
| GET | `/api/departemen/:id` | Detail departemen |
| PUT | `/api/departemen/:id` | Update departemen |
| DELETE | `/api/departemen/:id` | Hapus departemen (gagal jika masih direferensikan karyawan → 409) |
| POST | `/api/karyawan` | Tambah karyawan |
| GET | `/api/karyawan` | List seluruh karyawan |
| GET | `/api/karyawan/:id` | Detail karyawan |
| PUT | `/api/karyawan/:id` | Update data karyawan |
| DELETE | `/api/karyawan/:id` | Soft delete (status → `nonaktif`) |
| POST | `/api/karyawan/:id/komponen-gaji` | Tambah komponen gaji |
| GET | `/api/karyawan/:id/komponen-gaji` | List komponen gaji karyawan (terurut nominal, sorting manual) |
| GET | `/api/komponen-gaji/:id` | Detail komponen gaji |
| PUT | `/api/karyawan/:id/komponen-gaji/:komponen_id` | Update komponen gaji |
| POST | `/api/payroll/generate` | Generate payroll (draft), body JSON `{karyawan_id, periode}` |
| GET | `/api/payroll/:karyawan_id/riwayat` | Riwayat payroll karyawan |
| GET | `/api/payroll/laporan?periode=YYYY-MM-DD` | Laporan agregat payroll per departemen |


### Konvensi Format
- **Tanggal:** `YYYY-MM-DD` (date-only), bukan RFC3339 — konsisten di seluruh request/response.
- **Error response:** `{"error": "pesan dalam bahasa Indonesia"}`. Mapping status: `400` validasi/FK gagal, `404` resource tidak ditemukan, `409` duplikasi (UNIQUE constraint), `500` kegagalan tak terduga.

---

## Dokumentasi Tambahan
| Dokumen | Isi |
|---|---|
| `docs/erd.png` | Entity Relationship Diagram — kardinalitas relasi antar entitas |
| `docs/lrs.png` | Logical Record Structure — struktur tabel fisik, FK, tipe data |
| `docs/skalabilitas.md` | Analisis skalabilitas, index strategy, connection pooling |
| `docs/code-review-checklist.md` | Checklist review, temuan, pengecualian desain disengaja |
| `docs/debugging-log.md` | 13 kasus bug — gejala, root cause, fix, verifikasi; termasuk verifikasi tambahan pakai Delve untuk 2 kasus terpilih |
| `docs/profiling-report.md` | Hasil `pprof`, perbandingan before/after optimasi |
| `docs/lembar-pengujian-unit.md` | Dokumen formal pengujian unit |
| `docs/lembar-pengujian-integrasi.md` | Dokumen formal pengujian integrasi |
| `docs/data-provenance.md` | Proses analisis & transformasi dataset publik → skema |
---

## Environment Variables (`.env`)

| Variabel | Keterangan | Contoh/Default |
|---|---|---|
| `APP_PORT` | Port server aplikasi | `8080` |
| `DB_HOST` | Host PostgreSQL | `localhost` |
| `DB_PORT` | Port PostgreSQL | `5432` |
| `DB_NAME` | Nama database utama | `payroll_db` |
| `DB_USER` | Username database (bukan superuser) | `payroll_app` |
| `DB_PASSWORD` | Password user database | — |

Lihat `.env.example` untuk daftar lengkap & nilai default aktual.

---

## Troubleshooting

| Masalah | Solusi |
|---|---|
| **Koneksi database gagal** | Cek `.env` — pastikan `DB_USER`/`DB_PASSWORD` sesuai user yang dibuat lewat `migrations/setup-db-user-privilege-terbatas.sql`, bukan `postgres` |
| **`permission denied` saat query** | User `payroll_app` sengaja tanpa privilege `DROP`/`CREATE` — pastikan hanya migration yang dijalankan sebagai `postgres`, aplikasi tetap pakai `payroll_app` |
| **Error `002_sql_native_features.sql` gagal dijalankan** | Pastikan `001_init_schema.sql` sudah dijalankan lebih dulu — `002` bergantung pada tabel yang dibuat di `001` |
| **`UNIQUE constraint` gagal saat generate payroll** | Payroll untuk kombinasi `karyawan_id` + `periode` tersebut sudah pernah dibuat — cek `GET /api/payroll/:karyawan_id/riwayat`, ini business rule yang disengaja (409 Conflict) |
| **Port 8080 sudah dipakai** | Ubah `APP_PORT` di `.env`, restart aplikasi |
| **Integration test gagal di tahap setup (`TestMain`)** | Pastikan `payroll_test_db` sudah ada dan migration sudah dijalankan ke database test tersebut, bukan hanya ke database utama — lihat Bug #12 di `docs/debugging-log.md` |
| **Response tanggal muncul dalam format RFC3339, bukan `YYYY-MM-DD`** | Sudah diperbaiki (Bug #10), pastikan pakai build terbaru |

---

## Dokumentasi Kode (Godoc)

Seluruh exported function/struct memiliki godoc comment. Untuk generate/lihat:
```bash
go doc -all ./internal/service
go doc -all ./internal/repository
go doc -all ./internal/handler
```
Atau untuk fungsi/struct spesifik:
```bash
go doc ./internal/service PayrollService
```
