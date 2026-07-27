# docs/code-review-checklist.md

**Scope Review:** 
- `internal/service/*.go`
- `internal/repository/*.go`
- `internal/handler/*.go` + `bind_error.go`
- `internal/model/*.go`
- `internal/util/sort.go`
- `internal/config/config.go`

**Belum mencakup** `router.go` dan `cmd/api/main.go`.

**Metode:** Self-review, dilakukan setelah implementasi fase 1 selesai (handler + router + wiring berjalan), bukan sebelum menulis kode.

**Revisi:** Dokumen ini adalah revisi ke-2. Revisi pertama memuat 2 ketidakakuratan yang ditemukan lewat cross-check manual terhadap kode aktual (lihat catatan di Temuan #1 dan #2), dan 1 gap yang terlewat saat review awal (Temuan #6, baru). Seluruh 6 temuan pada revisi ini **sudah diperbaiki** di kode.

---

## Ringkasan

- **6 temuan** inkonsistensi implementasi ditemukan sepanjang proses review — **seluruhnya sudah diperbaiki**. Severity tertinggi saat ditemukan: **Medium** (1 temuan, sudah diverifikasi aman), sisanya **Low**.
- **2 pengecualian desain disengaja** (bukan bug) — sudah didokumentasikan alasannya.
- Pada file yang direview: **tidak ditemukan** celah SQL injection, kebocoran error mentah ke client, atau pelanggaran dependency flow.
- Tidak ada temuan **High**.

---

## 1. Checklist per Kategori

| # | Kategori | Status | Severity Tertinggi | Catatan |
|---|---|---|---|---|
| 1 | Naming Convention | Pass | — | Tidak ditemukan penyimpangan pada file yang direview |
| 2 | Error Handling | Ditemukan & Diperbaiki | Low | Lihat Temuan #1, #6 |
| 3 | SQL Injection Safety | Pass | — | Seluruh query di file yang direview parameterized |
| 4 | Konsistensi Struct/DTO | Ditemukan & Diperbaiki | Low | Lihat Temuan #2, #3 |
| 5 | Dependency Flow | Pass | — | Tidak ditemukan penyimpangan pada file yang direview |
| 6 | Godoc | Pass | — | Tidak ditemukan penyimpangan pada file yang direview |

Kriteria detail tiap kategori:

| # | Kategori | Kriteria |
|---|---|---|
| 1 | Naming Convention | Exported identifier PascalCase, unexported camelCase; nama struct/interface deskriptif; nama file `snake_case.go` konsisten dengan isi |
| 2 | Error Handling | Error dari layer bawah (DB) tidak bocor mentah ke client; sentinel error domain dipakai untuk kondisi yang perlu dibedakan; error non-domain dibungkus context |
| 3 | SQL Injection Safety | Seluruh query parameterized (`$1, $2, ...`), tidak ada string concatenation/`fmt.Sprintf` untuk nilai user |
| 4 | Konsistensi Struct/DTO | Response API pakai DTO terpisah, tidak expose `model.X` mentah; symmetric antar handler untuk operasi sejenis |
| 5 | Dependency Flow | Handler tidak akses `repository`/DB langsung; service tidak import `gin`/HTTP; layering `handler → service → repository` konsisten |
| 6 | Godoc | Semua exported func/struct/interface punya comment yang menjelaskan *apa* dan *kenapa* |

Catatan tambahan kategori 2 & 3: sentinel error domain (`ErrXxxNotFound`, dst) sudah dipakai konsisten dan dipetakan ke HTTP status yang tepat; tidak ada pesan error mentah PostgreSQL/validator yang bocor ke client (dibuktikan lewat fix Bug #1–#5 di `debugging-log.md`). 

Kategori 5: `payroll_service.go` bergantung pada `repository.PayrollRepository` untuk `BeginTx` (lewat kontrak interface), bukan akses pgx langsung — tetap sesuai pola. Handler mengimpor package `repository` **hanya** untuk membandingkan sentinel error (`errors.Is`) saat pemetaan ke HTTP status — bukan untuk akses data langsung, jadi tidak melanggar dependency flow.

---

## 2. Temuan Self-Review

Format: **Temuan → Dampak → Rekomendasi → Status**.

### Temuan #1 — Severity: Low - Fixed 
- **Kategori:** Error Handling
- **Lokasi:** `komponen_gaji_repository.go` (`GetByID`, `GetByKaryawanID`, `Update`), `payroll_repository.go` (`GetRiwayatByKaryawanID`, `GetLaporanAgregat`)
- **Temuan:** `departemen_repository.go`/`karyawan_repository.go` membungkus error non-mapped dengan `fmt.Errorf("gagal ...: %w", err)`; method-method di atas sebagian besar `return err`/`return nil, err` tanpa wrapping.
- **Koreksi terhadap draft awal:** Draft pertama dokumen ini keliru memasukkan `Update` (`komponen_gaji_repository.go`) sebagai "tidak wrapped". Setelah dicek ulang terhadap kode aktual, method itu **memang sudah** membungkus error — tapi dengan pesan yang salah copy-paste dari `Create()` (`"gagal membuat..."` alih-alih `"gagal update..."`), sehingga log debugging jadi menyesatkan saat operasi update gagal. Ini kesalahan yang berbeda dari "unwrapped", dan sudah diperbaiki (lihat Fix).
- **Dampak:** Saat debugging lewat log, error dari method-method di atas tidak menyebutkan operasi/ID yang gagal, atau (khusus `Update`) menyebutkan operasi yang salah — lebih sulit/menyesatkan untuk ditelusuri.
- **Fix:**
  - `komponen_gaji_repository.go Update`: pesan diperbaiki dari `"gagal membuat komponen gaji..."` → `"gagal update komponen gaji..."`.
  - `komponen_gaji_repository.go GetByID`/`GetByKaryawanID`, `payroll_repository.go GetRiwayatByKaryawanID`/`GetLaporanAgregat`: seluruh `return nil, err` generik dibungkus `fmt.Errorf("gagal ...: %w", err)`, konsisten dengan pola di `departemen_repository.go`/`karyawan_repository.go`. Sentinel error yang sudah ada (`ErrKomponenGajiNotFound`, dst.) tetap dikembalikan apa adanya, tidak dibungkus.

### Temuan #2 — Severity: Low - Fixed
- **Kategori:** Konsistensi Struct/DTO
- **Lokasi:** `komponen_gaji_handler.go`
- **Temuan (diperluas dari draft awal):** Draft pertama hanya menyebut `Create` dan `Update` yang mengembalikan `model.KomponenGaji` mentah lewat `c.JSON`. Setelah dicek ulang, **seluruh 4 endpoint** di file ini (`Create`, `Update`, `GetByID`, `GetByKaryawanID`) mengembalikan model mentah — bukan cuma 2. `karyawan_handler.go`/`payroll_handler.go` selalu memetakan ke DTO response terpisah.
- **Dampak:** API contract tidak konsisten antar entitas; perubahan struktur `model.KomponenGaji` di masa depan bisa langsung mengubah response tanpa lapisan pemetaan eksplisit.
- **Fix:** Ditambahkan `komponenGajiResponse`, `newKomponenGajiResponse`, dan `listKomponenGajiResponse`. Keempat endpoint diubah untuk mengembalikan DTO ini, konsisten dengan pola `karyawan_handler.go`.

### Temuan #3 — Severity: Low - Fixed
- **Kategori:** Konsistensi Struct/DTO
- **Lokasi:** `departemen_handler.go`
- **Temuan:** `Update` hanya mengembalikan `{"message": "..."}`, tidak mengembalikan data hasil — berbeda dari `Karyawan.Update`. (`Create`/`GetByID`/`GetAll` mengembalikan `model.Departemen` mentah, tidak masuk kategori masalah karena struktur model sederhana dan memang idealnya ditampilkan apa adanya. `Delete` sengaja tetap `{"message": "..."}` karena tidak ada data tersisa untuk dikembalikan — bukan bagian dari temuan ini.)
- **Dampak:** Client tidak tahu nilai final tanpa melakukan GET ulang setelah `Update`.
- **Fix:** `departemen_repository.go Update()` diubah dari `Exec` menjadi `QueryRow(...).Scan(&d.CreatedAt)` dengan `RETURNING created_at`, ditambah pengecekan `pgx.ErrNoRows` → `ErrDepartemenNotFound`. Handler `Update` sekarang mengembalikan `c.JSON(http.StatusOK, dept)`, konsisten dengan `Karyawan.Update`. Frontend (`api.js`) tidak terdampak selama tidak membaca `res.data.message` secara spesifik.

### Temuan #4 — Severity: Low - Fixed
- **Kategori:** Error Handling
- **Lokasi:** `payroll_repository.go`
- **Temuan:** Pesan `ErrPayrollAlreadyExists` memakai tanda seru ("...periode ini!"), beda gaya dari sentinel error lain yang tidak pakai tanda baca penekanan.
- **Dampak:** Inkonsistensi gaya pesan error, murni kosmetik, tidak berpengaruh pada logic atau parsing.
- **Fix:** Tanda seru dihapus → `"payroll sudah ada untuk karyawan dan periode ini"`.

### Temuan #5 — Severity: Medium - Diverifikasi, aman
- **Kategori:** Error Handling / Validasi
- **Lokasi:** `karyawan_handler.go`, `komponen_gaji_handler.go`
- **Temuan:** `binding:"required"` pada field `decimal.Decimal` (`GajiPokok`, `Nominal`) belum diverifikasi eksplisit apakah menolak nilai `0`.
- **Hasil verifikasi:**
  - **`gaji_pokok`** (karyawan): mengirim `0` menghasilkan `400 Bad Request` — `"gaji pokok tidak boleh nol"`. Namun proteksi ini berasal dari **validasi manual di service layer**, bukan dari tag `binding:"required"` — tag tersebut tidak terbukti menolak zero-value struct `decimal.Decimal` dengan sendirinya. Tag dipertahankan sebagai lapisan cek "field ada di JSON", validasi nilai tetap tanggung jawab service.
  - **`nominal`** (komponen_gaji): nilai `0` **sengaja diloloskan**, bukan bug. Karena `KomponenGajiRepository` tidak punya `Delete` (lihat Pengecualian #1, section 3), set `nominal = 0` adalah mekanisme resmi untuk "menonaktifkan" tunjangan/potongan tanpa hard-delete, menjaga jejak data tetap tertelusuri. Yang ditolak hanya nilai negatif (`ErrNominalNegatif`) — sudah pass test.
- **Dampak:** Tidak ada risiko fungsional. Kesalahpahaman awal (menyamakan perilaku `gaji_pokok` dan `nominal`) diperbaiki di catatan ini agar tidak disalahartikan assessor sebagai inkonsistensi validasi antar-field.
- **Rekomendasi:** Tidak ada tindakan lebih lanjut diperlukan.

### Temuan #6 — Severity: Low → Fixed (baru, ditemukan lewat cross-check)
- **Kategori:** Error Handling
- **Lokasi:** `payroll_repository.go` `Create()`
- **Temuan:** Gap yang terlewat pada review awal (section 2 draft pertama tidak menyebutkan method ini sama sekali, walau pola masalahnya identik dengan Temuan #1). Branch generic error pada `Create()` melakukan `return err` tanpa wrapping, hanya branch pelanggaran unique constraint (`23505` → `ErrPayrollAlreadyExists`) yang ditangani secara eksplisit.
- **Dampak:** Sama seperti Temuan #1 — error generik dari `Create()` (mis. koneksi terputus, tipe data mismatch di level DB) tidak menyebutkan konteks operasi saat muncul di log.
- **Fix:** Branch fallback dibungkus `fmt.Errorf("gagal insert payroll: %w", err)`.

---

## 3. Pengecualian Coding-Guideline (Disengaja)

Guideline umum project ini: setiap entitas idealnya punya CRUD simetris dan perilaku delete yang konsisten. Dua kasus berikut sengaja menyimpang — ini **bukan** temuan yang perlu diperbaiki (beda kategori dari section 2).

### Pengecualian #1 — `KomponenGajiService`/`KomponenGajiRepository` Tanpa Method `Delete`
- **Guideline yang dilanggar:** CRUD lengkap per entitas.
- **Alasan:** Komponen gaji yang sudah pernah dipakai dalam generate payroll historis (`payroll.total_tunjangan`/`total_potongan` adalah snapshot hasil kalkulasi darinya) tidak boleh hilang jejaknya. Jika HR salah input nominal/persentase, koreksi dilakukan lewat `Update`, bukan hapus-lalu-buat-ulang, supaya data tetap bisa ditelusuri. Sebagai konsekuensi langsung, `nominal = 0` berfungsi sebagai mekanisme "nonaktifkan" komponen (lihat Temuan #5).
- **Catatan implementasi:** Kode `Delete` yang di-comment-out di `komponen_gaji_repository.go` sengaja dibiarkan sebagai dokumentasi keputusan ini, bukan dihapus total.

### Pengecualian #2 — Hard-Delete Departemen vs Soft-Delete Karyawan
- **Guideline yang dilanggar:** Pola delete konsisten di semua entitas.
- **Alasan:** Departemen adalah master data organisasi murni tanpa histori transaksional yang melekat langsung padanya, sehingga aman dihapus permanen (dilindungi FK constraint jika masih direferensikan — lihat Bug #1 di `debugging-log.md`). Karyawan terhubung ke riwayat `payroll` yang harus tetap bisa ditelusuri untuk audit, sehingga hanya dinonaktifkan (`status = 'nonaktif'`), tidak pernah dihapus fisik.

---

## 4. Kesimpulan

Self-review mencakup file yang disebutkan di scope (model → repository → service → handler), **belum** mencakup `router.go`/`main.go`. Ditemukan 2 pengecualian desain disengaja (section 3) dan 6 temuan inkonsistensi implementasi (section 2) — 5 kosmetik/error-handling (Low), 1 berpotensi fungsional yang telah diverifikasi aman (Medium, Temuan #5). Seluruh 6 temuan **sudah diperbaiki** di kode per tanggal revisi dokumen ini. Pada file yang direview: tidak ditemukan celah SQL injection, kebocoran pesan error mentah ke client (diverifikasi lewat Bug #1–#5 di `debugging-log.md`), atau pelanggaran dependency flow `handler → service → repository`.

**Catatan proses:** Dokumen draft pertama memuat 2 ketidakakuratan (Temuan #1 salah mengategorikan `Update` sebagai unwrapped, padahal wrapped-dengan-pesan-salah; Temuan #2 undercount cakupan endpoint) dan 1 gap (Temuan #6 belum tercatat sama sekali). Ketiganya ditemukan lewat cross-check manual baris-per-baris terhadap kode aktual, bukan dari review awal. Dicatat di sini sebagai bukti proses verifikasi berlapis, bukan untuk disembunyikan.
