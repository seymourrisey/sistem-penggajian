# Project Design — Sistem Informasi Penggajian

**Studi Kasus:** Analis Sistem — Rancang & Implementasi Aplikasi Perangkat Lunak
**Tech Stack:** Golang (Gin), PostgreSQL (pgAdmin4), Layered Architecture
**Tujuan:** Bukti kompetensi 10 unit skema SKKNI (J.620100.xxx)

---

## 1. Requirements

### 1.1 Functional Requirements
| ID | Requirement |
|---|---|
| F1 | CRUD data karyawan (create, read, update, soft-delete) |
| F2 | CRUD data departemen (create, read, update, hard-delete) |
| F3 | Kelola komponen gaji (tunjangan & potongan) per karyawan |
| F4 | Generate slip gaji per karyawan per periode (gaji pokok + tunjangan − potongan) |
| F5 | Lihat riwayat gaji per karyawan |
| F6 | Laporan agregat gaji per departemen per periode |
| F7 | Menampilkan daftar komponen gaji karyawan terurut berdasarkan nominal (algoritma sorting manual, bukan `ORDER BY` SQL) |

### 1.2 Non-Functional Requirements
| ID | Requirement | Terkait Kompetensi |
|---|---|---|
| NF1 | Response time API < 200ms untuk single record | Skalabilitas |
| NF2 | Schema siap untuk index & connection pooling | Skalabilitas |
| NF3 | Kode terdokumentasi (godoc di-generate via `go doc`/`godoc`, bukan sekadar ditulis) + README | Dokumen kode |
| NF4 | Struktur mendukung unit test & integration test terpisah, disertai lembar pengujian formal (bukan hanya kode test) | Pengujian |
| NF5 | Koneksi database menggunakan user PostgreSQL dengan privilege terbatas (bukan superuser), transaksi eksplisit (COMMIT/ROLLBACK) untuk operasi kritis | Akses basis data |
| NF6 | Debugging menggunakan tools eksplisit (Delve debugger dan/atau structured logging), dicatat di `docs/debugging-log.md`. **Realisasi:** metode utama adalah exploratory testing manual via Postman (13 kasus tercatat); Delve digunakan sebagai verifikasi tambahan pada 2 kasus representatif (root cause sudah diketahui, direproduksi ulang untuk konfirmasi via breakpoint & inspect variable) — lihat section "Verifikasi Tambahan dengan Delve" di `docs/debugging-log.md` | Debugging |

### 1.3 Batasan Scope (out of scope, dan alasannya)
- **Tidak ada PPh21** — butuh tarif progresif + PTKP, kompleksitas hukum di luar scope studi kasus. Cukup BPJS (Kesehatan + Ketenagakerjaan) sebagai komponen potongan tetap.
- **Tidak ada autentikasi/role-permission kompleks** — fokus proyek pada akses data & kalkulasi, bukan security layer.
- **Tidak ada UI/frontend** — deliverable berupa REST API, presentasi cukup lewat Postman/dokumentasi.
- **Fitur SQL native (section 2.4) dibuat minimal 1 contoh per jenis** (1 function, 1 trigger, 1 view, 1 stored procedure) — cukup untuk memenuhi KUK, tidak perlu diterapkan di semua endpoint atau menggantikan logic Go yang sudah ada.
- **Algoritma sorting (F7) dibuat sederhana** (misal insertion sort) hanya di satu tempat (list komponen gaji) — tidak perlu custom sorting di semua endpoint list lain yang sudah cukup terwakili oleh `ORDER BY` SQL.

---

## 2. Database Design

### 2.1 Schema

```sql
CREATE TABLE departemen (
    id          SERIAL PRIMARY KEY,
    nama        VARCHAR(100) NOT NULL UNIQUE,
    created_at  TIMESTAMP DEFAULT NOW()
);

CREATE TABLE karyawan (
    id              SERIAL PRIMARY KEY,
    nip             VARCHAR(20) NOT NULL UNIQUE,
    nama            VARCHAR(100) NOT NULL,
    departemen_id   INT NOT NULL REFERENCES departemen(id),
    jabatan         VARCHAR(50) NOT NULL,
    gaji_pokok      NUMERIC(12,2) NOT NULL CHECK (gaji_pokok >= 0),
    tanggal_masuk   DATE NOT NULL,
    status          VARCHAR(20) DEFAULT 'aktif',
    created_at      TIMESTAMP DEFAULT NOW(),
    updated_at      TIMESTAMP DEFAULT NOW()
);

CREATE TABLE komponen_gaji (
    id          SERIAL PRIMARY KEY,
    karyawan_id INT NOT NULL REFERENCES karyawan(id),
    jenis       VARCHAR(20) NOT NULL CHECK (jenis IN ('tunjangan','potongan')),
    nama        VARCHAR(50) NOT NULL,
    nominal     NUMERIC(12,2) NOT NULL CHECK (nominal >= 0),
    is_persen   BOOLEAN DEFAULT FALSE

    CONSTRAINT uq_komponen_gaji_karyawan_jenis_nama
            UNIQUE (karyawan_id, jenis, nama)
);

CREATE TABLE payroll (
    id              SERIAL PRIMARY KEY,
    karyawan_id     INT NOT NULL REFERENCES karyawan(id),
    periode         DATE NOT NULL,
    gaji_pokok      NUMERIC(12,2) NOT NULL,
    total_tunjangan NUMERIC(12,2) NOT NULL,
    total_potongan  NUMERIC(12,2) NOT NULL,
    gaji_bersih     NUMERIC(12,2) NOT NULL,
    status          VARCHAR(20) DEFAULT 'draft',
    created_at      TIMESTAMP DEFAULT NOW(),
    UNIQUE(karyawan_id, periode)
);

CREATE INDEX idx_karyawan_departemen ON karyawan(departemen_id);
CREATE INDEX idx_payroll_karyawan_periode ON payroll(karyawan_id, periode);
CREATE INDEX idx_komponen_karyawan ON komponen_gaji(karyawan_id);
```

### 2.2 Justifikasi Desain
- **Kolom `status` pada `karyawan` hanya boleh berubah lewat soft-delete** (`DELETE /api/karyawan/:id`), bukan lewat `PUT /api/karyawan/:id` (update data biasa). Ini business rule sengaja, bukan keterbatasan teknis: `status` adalah state transition (aktif → nonaktif), beda kategori dari field koreksi data seperti `nama`/`jabatan`/`gaji_pokok`. Kalau `PUT` juga bisa ubah `status`, ada dua jalur berbeda yang bisa mengubah field yang sama — inkonsisten dengan desain endpoint `DELETE` di atas. Di level repository, query `UPDATE` untuk data biasa sengaja tidak menyertakan kolom `status` sama sekali (bukan sekadar divalidasi di service, tapi secara struktural tidak bisa diubah lewat jalur itu).
- **Normalisasi `komponen_gaji`**: dipisah dari `karyawan` agar satu karyawan bisa punya banyak tunjangan/potongan tanpa mengubah struktur tabel utama (1-to-many).
- **Snapshot pattern di `payroll`**: nilai gaji_pokok, total_tunjangan, dst disimpan sebagai snapshot, bukan hanya foreign key — supaya perubahan gaji pokok bulan berjalan tidak mengubah riwayat bulan sebelumnya (data integrity historis).
- **Index**: ditempatkan pada kolom yang digunakan di WHERE/JOIN (`departemen_id`, `karyawan_id + periode`) — persiapan langsung untuk skalabilitas query saat data bertambah besar.
- **UNIQUE(karyawan_id, periode)**: mencegah duplikasi generate slip gaji pada periode yang sama.
- **decimal.Decimal untuk nilai uang**: dipilih agar perhitungan nominal tetap presisi (menghindari error floating point) sekaligus mendukung binding JSON numerik secara langsung tanpa perlu workaround parsing string manual.
- **Departemen menggunakan hard-delete, karyawan menggunakan soft-delete** — perbedaan ini disengaja, bukan inkonsistensi. Departemen adalah master data organisasi murni tanpa histori transaksional yang melekat langsung padanya (sekadar label pengelompokan), sehingga dapat dihapus permanen jika belum/tidak lagi direferensikan karyawan. Karyawan sebaliknya terhubung ke riwayat payroll yang harus tetap dapat ditelusuri (audit trail), sehingga statusnya hanya dinonaktifkan, tidak dihapus. Integritas data departemen tetap terjaga melalui foreign key constraint pada tabel `karyawan` — departemen yang masih direferensikan tidak dapat dihapus dan database akan mengembalikan error.

### 2.3 Diagram

### Entity Relationship Diagram
![Entity Relationship Diagram](/docs/diagram-erd.png)

### Logical Record Structure
![Logical Record Structure](/docs/diagram-lrs.png)

### Class Diagram
![Class Diagram](/docs/diagram-class.png)

### 2.4 Fitur SQL Native (Stored Procedure, Function, Trigger, View, Transaksi)

Ditambahkan khusus untuk memenuhi KUK unit kompetensi #2 (Menggunakan SQL) yang eksplisit meminta stored procedure, function, trigger, view, dan commit/rollback — di luar query DML dasar yang sudah ada di repository layer. Fitur ini **melengkapi**, bukan menggantikan, implementasi logic di Go (`payroll_service.go` tetap jadi source-of-truth kalkulasi untuk keperluan unit test).

```sql
-- FUNCTION: hitung gaji bersih (versi SQL, demonstrasi paralel dari logic Go)
CREATE OR REPLACE FUNCTION fn_hitung_gaji_bersih(
    p_gaji_pokok NUMERIC, p_tunjangan NUMERIC, p_potongan NUMERIC
) RETURNS NUMERIC AS $$
BEGIN
    RETURN p_gaji_pokok + p_tunjangan - p_potongan;
END;
$$ LANGUAGE plpgsql;

-- TRIGGER: auto-update updated_at pada karyawan saat UPDATE
CREATE OR REPLACE FUNCTION trg_set_updated_at() RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_karyawan_updated_at
BEFORE UPDATE ON karyawan
FOR EACH ROW EXECUTE FUNCTION trg_set_updated_at();

-- VIEW: laporan gaji per departemen (menyederhanakan query JOIN+GROUP BY yang sudah ada)
CREATE VIEW v_laporan_gaji_departemen AS
SELECT d.nama AS departemen, p.periode,
       COUNT(p.id) AS jumlah_karyawan,
       SUM(p.gaji_bersih) AS total_gaji_bersih,
       AVG(p.gaji_bersih) AS rata_rata_gaji
FROM payroll p
JOIN karyawan k ON p.karyawan_id = k.id
JOIN departemen d ON k.departemen_id = d.id
GROUP BY d.nama, p.periode;

-- STORED PROCEDURE: contoh generate payroll snapshot via SQL murni (demonstrasi tambahan)
CREATE OR REPLACE PROCEDURE sp_generate_payroll_snapshot(
    p_karyawan_id INT, p_periode DATE
)
LANGUAGE plpgsql AS $$
DECLARE
    v_gaji_pokok NUMERIC;
BEGIN
    SELECT gaji_pokok INTO v_gaji_pokok FROM karyawan WHERE id = p_karyawan_id;
    INSERT INTO payroll (karyawan_id, periode, gaji_pokok, total_tunjangan, total_potongan, gaji_bersih)
    VALUES (p_karyawan_id, p_periode, v_gaji_pokok, 0, 0, v_gaji_pokok);
END;
$$;
```

**Transaksi eksplisit (COMMIT/ROLLBACK)**: proses `GeneratePayroll` di `payroll_service.go` dibungkus dalam satu transaksi pgx (`tx.Begin()` → insert payroll → `tx.Commit()`; jika terjadi error di tengah proses, `tx.Rollback()`), supaya insert payroll bersifat atomik dan tidak meninggalkan data parsial jika terjadi kegagalan.

**Keamanan koneksi & hak akses (KUK unit #3)**: aplikasi terhubung ke PostgreSQL menggunakan user database dedicated (bukan `postgres` superuser) dengan privilege terbatas hanya pada schema aplikasi (`GRANT SELECT, INSERT, UPDATE, DELETE` pada tabel terkait, tanpa `DROP`/`CREATE`). Koneksi lokal menggunakan `sslmode=disable` untuk keperluan development; jika deployment ke server terpisah, `sslmode=require` wajib diaktifkan.

---

## 3. Architecture

### 3.1 Pattern
Layered Architecture: **Handler → Service → Repository → Model**
- **Handler**: terima HTTP request, validasi input dasar, panggil service
- **Service**: business logic (kalkulasi gaji, validasi bisnis)
- **Repository**: akses database (query SQL)
- **Model**: struct representasi data

### 3.2 Folder Structure
```
sistem-penggajian/
├── cmd/api/main.go
├── internal/
│   ├── config/config.go
│   ├── model/
│   │   ├── karyawan.go
│   │   ├── departemen.go
│   │   ├── komponen_gaji.go
│   │   ├── laporan.go
│   │   └── payroll.go
│   ├── repository/
│   │   ├── karyawan_repository.go
│   │   ├── departemen_repository.go
│   │   ├── komponen_gaji_repository.go
│   │   └── payroll_repository.go
│   ├── service/
│   │   ├── karyawan_service.go
│   │   ├── departemen_service.go
│   │   ├── komponen_gaji_service.go
│   │   └── payroll_service.go
│   ├── handler/
│   │   ├── karyawan_handler.go
│   │   ├── departemen_handler.go
│   │   ├── komponen_gaji_handler.go
│   │   ├── bind_error.go
│   │   └── payroll_handler.go
│   ├── util/
│   │   └── sort.go
│   └── router/router.go
├── pkg/database/postgres.go
├── migrations/
│   ├── 001_init_schema.sql
│   ├── 002_sql_native_features.sql
│   ├── setup-db-user-privilege-terbatas.sql
│   └── seed.sql
├── tests
│   ├── integration
│   │   ├── departemen_delete_test.go
│   │   ├── karyawan_api_test.go
│   │   ├── karyawan_create_test.go
│   │   ├── karyawan_getbyid_test.go
│   │   ├── karyawan_softdelete_test.go
│   │   ├── karyawan_update_test.go
│   │   ├── komponen_gaji_list_test.go
│   │   ├── payroll_generate_test.go
│   │   └── payroll_riwayat_test.go
│   └── unit
│       ├── payroll_service_test.go
│       ├── sort_bench_test.go
│       └── sort_test.go
├── docs/
│   ├── skalabilitas.md
│   ├── code-review-checklist.md
│   ├── profiling-report.md
│   ├── debugging-log.md
│   ├── lembar-pengujian-unit.md
│   ├── lembar-pengujian-integrasi.md
│   └── data-provenance.md
├── .env.example
├── go.mod
└── README.md
```

### 3.3 Alasan Struktur
- **`komponen_gaji_service.go`** dan **`komponen_gaji_handler.go`**: ditambahkan saat implementasi (di luar listing awal), diperlukan supaya endpoint komponen-gaji tetap mengikuti dependency flow `handler → service → repository` (section 3.1) dan tidak mengakses `KomponenGajiRepository` langsung dari handler. Scope service ini mencakup `Create`, `GetByKaryawanID`, `GetByID`, dan `Update` — mengikuti 4 endpoint komponen-gaji di section 4 (tambah, list per karyawan, detail, dan koreksi data). Sengaja **tidak** menyediakan `Delete` — komponen gaji yang sudah pernah dipakai dalam generate payroll historis sebaiknya dikoreksi lewat `Update` (mis. HR salah input nominal/persentase), bukan dihapus, untuk menjaga jejak data tetap dapat ditelusuri.
- `internal/` tidak bisa di-import project lain → proteksi arsitektur.
- `pkg/database` reusable, dipisah dari internal logic.
- `tests/unit` dan `tests/integration` dipisah eksplisit → mapping langsung ke unit kompetensi pengujian.
- `docs/` di root → artefak non-kode (skalabilitas, code review, profiling) mudah ditemukan assessor tanpa membaca kode.
- `internal/handler/bind_error.go` berisi helper untuk translate error bind JSON request.
- `internal/util/sort.go` berisi implementasi algoritma sorting manual (bukan `ORDER BY` SQL) untuk mengurutkan daftar komponen gaji berdasarkan nominal — bukti konkret KUK unit #4 ("membuat algoritma untuk sorting") beserta catatan kompleksitas waktu (Big-O) di komentar kode.
- `migrations/002_sql_native_features.sql` memuat stored procedure, function, trigger, dan view (lihat section 2.4) — dipisah dari `001_init_schema.sql` supaya jelas mana DDL struktural vs fitur SQL tambahan untuk demonstrasi kompetensi.
- `docs/lembar-pengujian-unit.md` dan `docs/lembar-pengujian-integrasi.md` adalah dokumen formal terpisah dari kode test (`tests/unit`, `tests/integration`) — berisi tabel skenario, data uji, hasil, dan status pass/fail, sesuai KUK unit #9 dan #10 yang eksplisit meminta "lembar pengujian", bukan hanya kode.
- `docs/data-provenance.md` mendokumentasikan proses analisis dataset publik (sumber, profiling, evaluasi kesesuaian field terhadap skema, transformasi, dan alasan tiap keputusan) — bukti langsung proses kerja analis data terhadap instruksi asesor untuk mempelajari sifat data publik vs privat, mendukung unit kompetensi #1 (skalabilitas/analisis) dan #5 (dokumentasi).
---

## 4. API Endpoints

| Method | Endpoint | Fungsi | Query/Logic Highlight |
|---|---|---|---|
| POST | `/api/departemen` | Tambah departemen | INSERT |
| GET | `/api/departemen` | List seluruh departemen | SELECT |
| GET | `/api/departemen/:id` | Detail departemen | SELECT by Primary Key |
| PUT | `/api/departemen/:id` | Update departemen | UPDATE |
| DELETE | `/api/departemen/:id` | Hapus departemen | DELETE. Gagal jika masih direferensikan data karyawan (Foreign Key Constraint). |
| POST | `/api/karyawan` | Tambah karyawan | INSERT |
| GET | `/api/karyawan` | List seluruh karyawan | SELECT |
| GET | `/api/karyawan/:id` | Detail karyawan | SELECT by Primary Key |
| PUT | `/api/karyawan/:id` | Update data karyawan | UPDATE |
| DELETE | `/api/karyawan/:id` | Soft delete karyawan | UPDATE status menjadi `nonaktif` (data tidak dihapus secara fisik). |
| POST | `/api/karyawan/:id/komponen-gaji` | Tambah komponen gaji | INSERT. Komponen hanya dapat ditambahkan pada karyawan yang sudah ada. |
| GET | `/api/karyawan/:id/komponen-gaji` | List komponen gaji karyawan | SELECT berdasarkan `karyawan_id`. Hasil diurutkan berdasarkan nominal terbesar menggunakan algoritma sorting manual di `internal/util/sort.go` (bukan `ORDER BY` SQL) — bukti KUK unit #4. |
| GET | `/api/komponen-gaji/:id` | Detail komponen gaji | SELECT by Primary Key |
| PUT | `/api/karyawan/:id/komponen-gaji/:komponen_id` | Update komponen gaji | UPDATE berdasarkan `komponen_id` dan validasi bahwa komponen tersebut milik `karyawan_id` pada URL. Mendukung komponen nominal tetap maupun persentase (`is_persen`). |
| POST | `/api/payroll/generate` | Generate payroll | Mengambil data karyawan dan seluruh komponen gaji, menghitung total tunjangan dan potongan (nominal/persentase), menghitung gaji bersih, kemudian menyimpan payroll baru berstatus **draft**. Payload dikirim melalui JSON body (`karyawan_id`, `periode`), bukan URL parameter. Mencegah payroll ganda untuk kombinasi `karyawan_id` dan `periode`. |
| GET | `/api/payroll/:karyawan_id/riwayat` | Riwayat payroll karyawan | Validasi terlebih dahulu bahwa `karyawan_id` ada. Menggunakan **JOIN** tabel `payroll` dan `karyawan`. Jika `karyawan_id` tidak ditemukan → **404 Not Found**. Jika karyawan ada tetapi belum memiliki payroll → **200 OK** dengan array kosong (`[]`). |
| GET | `/api/payroll/laporan?periode=YYYY-MM-DD` | Laporan payroll agregat per departemen | Query agregasi menggunakan **JOIN** tabel `payroll`, `karyawan`, dan `departemen` disertai **GROUP BY**, **COUNT**, **SUM**, dan **AVG**. Parameter `periode` wajib dikirim sebagai query parameter. Jika periode tidak memiliki data payroll, mengembalikan **200 OK** dengan array kosong (`[]`). |

#### Payload `POST /api/payroll/generate`

```json
{
  "karyawan_id": 1,
  "periode": "2026-07-01"
}
```

Request menggunakan **JSON body**, bukan URL parameter, karena endpoint ini merepresentasikan proses generate payroll berdasarkan kombinasi data input.

### Payroll duplicate prevention

Pada bagian Payroll, tambahkan satu business rule:

"Sistem tidak mengizinkan lebih dari satu payroll untuk kombinasi karyawan dan periode yang sama. Validasi dilakukan melalui UNIQUE Constraint pada database, dan pelanggaran akan dikembalikan sebagai HTTP 409 Conflict."

### 4.1 Format Field Tanggal

Semua field bertipe tanggal di request/response JSON (`tanggal_masuk`, `periode`) menggunakan format **`YYYY-MM-DD`** (date-only, tanpa komponen jam/timezone) — bukan RFC3339 default Go. Konsisten dengan tipe kolom `DATE` (bukan `TIMESTAMP`) di schema section 2.1, sehingga tidak ada ambiguitas timezone saat parsing maupun penyimpanan.

Contoh: `{"tanggal_masuk": "2026-07-20"}`, query param `?periode=2026-07-01`.

### 4.2 Kontrak Error Response

Semua endpoint yang gagal mengembalikan body JSON dengan bentuk konsisten:

```json
{"error": "pesan error dalam bahasa Indonesia"}
```

Mapping HTTP status code:

| Status | Kondisi |
|---|---|
| 400 Bad Request | Validasi input gagal (field kosong, format salah) atau referensi FK tidak valid (mis. `departemen_id`/`karyawan_id` yang dirujuk tidak ada) |
| 404 Not Found | Resource yang diminta (by ID) tidak ditemukan, termasuk `GET /api/payroll/:karyawan_id/riwayat` jika `karyawan_id` tidak ada |
| 409 Conflict | Duplikasi yang melanggar UNIQUE constraint (nip, nama departemen, kombinasi karyawan_id+periode payroll) |
| 500 Internal Server Error | Kegagalan tak terduga di luar kategori di atas |


### 4.3 Dataset / Data Uji

Dataset menggunakan **kombinasi data publik (Kaggle) dan data sintetis terarah**, bukan data dummy murni. Sumber utama: [Employee Salary Dataset](https://www.kaggle.com/datasets/prince7489/employee-salary-dataset) (50 baris, kolom `Department`, `Experience_Years`, `Monthly_Salary`, dll).

Pendekatan ini dipilih untuk memenuhi instruksi panduan tugas ("siapkan dataset penggajian, dapat menggunakan data publik ataupun data privat") sekaligus arahan asesor untuk mempelajari sifat-sifat data publik vs privat sebelum digunakan. Field dari dataset sumber **tidak** dipetakan mentah-mentah ke skema — setiap field melalui evaluasi kesesuaian, dan field yang tidak relevan (`Name`, `Education_Level`, `Age`, `City`, `Gender`) sengaja tidak dipakai. Rincian lengkap proses evaluasi, transformasi, dan alasan tiap keputusan didokumentasikan di `docs/data-provenance.md`.

Ringkasan transformasi:
- `Department` → `departemen.nama` (dipakai apa adanya, 5 kategori)
- `Experience_Years` → `karyawan.jabatan` (tier: 0–2 Staff, 3–7 Staff Senior, 8–15 Manager, 16+ Senior Manager)
- `Monthly_Salary` (INR) → `karyawan.gaji_pokok` (IDR): **tidak** dikonversi kurs langsung, karena nilai INR tidak merepresentasikan skala gaji Indonesia. Sebagai gantinya, nilai di-transform jadi percentile rank (0–1) lalu di-remap linear ke rentang Rp5.000.000–Rp35.000.000 — mempertahankan *distribusi relatif* antar-karyawan dari data asli, bukan nilai absolutnya.
- `nip`, `nama`, `tanggal_masuk` — sintetis penuh (tidak tersedia di dataset sumber); `tanggal_masuk` diturunkan dari `Experience_Years` sebagai proxy tanggal masuk kerja.

Edge case sengaja disisipkan pada seed: karyawan berstatus `nonaktif` (uji filter soft-delete), dan satu karyawan tanpa `komponen_gaji` sama sekali (uji unit kalkulasi payroll saat komponen kosong).

> Catatan endpoint `GET /api/payroll/:karyawan_id/riwayat`:
> - `404 Not Found` apabila `karyawan_id` tidak ada.
> - `200 OK` dengan `[]` apabila karyawan ada tetapi belum memiliki riwayat payroll.


---

## 5. Mapping ke 10 Unit Kompetensi

| Unit Kompetensi | Bukti di Proyek |
|---|---|
| **1. Menganalisis skalabilitas perangkat lunak** | `docs/skalabilitas.md` — mencakup: lingkup sistem & lingkungan operasi (KUK elemen 1), analisis kompleksitas vs jumlah data/user, kebutuhan perangkat keras, index strategy, connection pooling (pgx pool), potensi bottleneck, opsi horizontal scaling |
| **2. Menggunakan SQL** | Query DML dasar di seluruh repository + query kompleks JOIN 3 tabel & GROUP BY di endpoint laporan + `migrations/002_sql_native_features.sql` (function, trigger, view, stored procedure — section 2.4) + transaksi eksplisit COMMIT/ROLLBACK di `GeneratePayroll` |
| **3. Menerapkan akses basis data** | Koneksi via `pgx` connection pool di `pkg/database/postgres.go`; user database dedicated dengan privilege terbatas (section 2.4); pengujian performa statement akses dicatat di `docs/profiling-report.md` |
| **4. Mengimplementasikan algoritma pemrograman** | Logic kalkulasi gaji di `payroll_service.go` (loop, kondisi, akumulasi) + algoritma sorting manual di `internal/util/sort.go` untuk urutan komponen gaji, disertai catatan kompleksitas waktu & memori |
| **5. Membuat dokumen kode program** | Godoc comment di semua exported function/struct (di-generate via `go doc`) + `README.md` |
| **6. Melakukan debugging** | `docs/debugging-log.md` — dicatat real-time, menyebutkan tools yang dipakai (Delve/structured logging), minimal 2-3 kasus dengan gejala/root cause/fix |
| **7. Melakukan profiling program** | `docs/profiling-report.md` — hasil `pprof` pada endpoint generate payroll & laporan, **perbandingan before/after** optimasi (mis. sebelum/sesudah index), interpretasi bottleneck |
| **8. Menerapkan code review** | `docs/code-review-checklist.md` — checklist standar + contoh temuan self-review + minimal 1 kasus pengecualian coding-guideline dengan komentar alasannya |
| **9. Melaksanakan pengujian unit program** | `tests/unit/payroll_service_test.go` (kode) + `docs/lembar-pengujian-unit.md` (dokumen formal: skenario, data uji, hasil, status) |
| **10. Melaksanakan pengujian integrasi program** | `tests/integration/karyawan_api_test.go` (kode) + `docs/lembar-pengujian-integrasi.md` (dokumen formal: peralatan, data uji, hasil, analisis kesesuaian) |

---
