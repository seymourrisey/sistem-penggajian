# docs/code-review-checklist.md

- **Scope Review:** `internal/service/*.go`, `internal/repository/*.go`, `internal/handler/*.go` + `bind_error.go`, `internal/model/*.go`, `internal/util/sort.go`, `internal/config/config.go`. **Belum mencakup** `router.go` dan `cmd/api/main.go`.
- **Metode:** Self-review, dilakukan setelah implementasi fase 1 selesai (handler + router + wiring berjalan), bukan sebelum menulis kode.

---

## Ringkasan

- **5 temuan** inkonsistensi implementasi (semua kosmetik/konsistensi, tidak mengubah behavior fungsional saat ini) — severity tertinggi: **Medium** (1 temuan), sisanya **Low**.
- **2 pengecualian desain disengaja** (bukan bug) — sudah didokumentasikan alasannya.
- Pada file yang direview: **tidak ditemukan** celah SQL injection, kebocoran error mentah ke client, atau pelanggaran dependency flow.
- Tidak ada temuan **High** — aman lanjut ke task berikutnya, 5 temuan Low/Medium bisa diperbaiki belakangan tanpa memblokir.

---

## 1. Checklist per Kategori

| # | Kategori | Status | Severity Tertinggi | Catatan |
|---|---|---|---|---|
| 1 | Naming Convention | ✅ Pass | — | Tidak ditemukan penyimpangan pada file yang direview |
| 2 | Error Handling | ⚠️ Ditemukan Masalah | Low | Lihat Temuan #1 |
| 3 | SQL Injection Safety | ✅ Pass | — | Seluruh query di file yang direview parameterized |
| 4 | Konsistensi Struct/DTO | ⚠️ Ditemukan Masalah | Low | Lihat Temuan #2, #3 |
| 5 | Dependency Flow | ✅ Pass | — | Tidak ditemukan penyimpangan pada file yang direview |
| 6 | Godoc | ✅ Pass | — | Tidak ditemukan penyimpangan pada file yang direview |

Kriteria detail tiap kategori:

| # | Kategori | Kriteria |
|---|---|---|
| 1 | Naming Convention | Exported identifier PascalCase, unexported camelCase; nama struct/interface deskriptif; nama file `snake_case.go` konsisten dengan isi |
| 2 | Error Handling | Error dari layer bawah (DB) tidak bocor mentah ke client; sentinel error domain dipakai untuk kondisi yang perlu dibedakan; error non-domain dibungkus context |
| 3 | SQL Injection Safety | Seluruh query parameterized (`$1, $2, ...`), tidak ada string concatenation/`fmt.Sprintf` untuk nilai user |
| 4 | Konsistensi Struct/DTO | Response API pakai DTO terpisah, tidak expose `model.X` mentah; symmetric antar handler untuk operasi sejenis |
| 5 | Dependency Flow | Handler tidak akses `repository`/DB langsung; service tidak import `gin`/HTTP; layering `handler → service → repository` konsisten |
| 6 | Godoc | Semua exported func/struct/interface punya comment yang menjelaskan *apa* dan *kenapa* |

Catatan tambahan kategori 2 & 3: sentinel error domain (`ErrXxxNotFound`, dst) sudah dipakai konsisten dan dipetakan ke HTTP status yang tepat; tidak ada pesan error mentah PostgreSQL/validator yang bocor ke client (dibuktikan lewat fix Bug #1–#5 di `debugging-log.md`). Kategori 5: `payroll_service.go` bergantung pada `repository.PayrollRepository` untuk `BeginTx` (lewat kontrak interface), bukan akses pgx langsung — tetap sesuai pola.

---

## 2. Temuan Self-Review

Format: **Temuan → Dampak → Rekomendasi**. Semua temuan di bawah bersifat kosmetik/konsistensi — tidak mengubah behavior fungsional saat ini, sengaja tidak diperbaiki di siklus ini kecuali diprioritaskan ulang, dicatat sebagai bukti proses review.

### Temuan #1 — Severity: Low
- **Kategori:** Error Handling
- **Lokasi:** `komponen_gaji_repository.go`, `payroll_repository.go`
- **Temuan:** `departemen_repository.go`/`karyawan_repository.go` membungkus error non-mapped dengan `fmt.Errorf("gagal ...: %w", err)`; `komponen_gaji_repository.go`/`payroll_repository.go` sebagian besar method (`GetByID`, `GetByKaryawanID`, `Update`, `GetRiwayatByKaryawanID`, `GetLaporanAgregat`) langsung `return err` tanpa wrapping.
- **Dampak:** Saat debugging lewat log, error dari dua file terakhir tidak menyebutkan operasi/ID yang gagal — lebih sulit ditelusuri dibanding dua file lainnya.
- **Rekomendasi:** Samakan pola: bungkus dengan `fmt.Errorf("gagal ...: %w", err)`.

### Temuan #2 — Severity: Low
- **Kategori:** Konsistensi Struct/DTO
- **Lokasi:** `komponen_gaji_handler.go`
- **Temuan:** `Create` dan `Update` mengembalikan `model.KomponenGaji` mentah lewat `c.JSON(..., k)`, sedangkan `karyawan_handler.go`/`payroll_handler.go` selalu memetakan ke DTO response terpisah.
- **Dampak:** API contract tidak konsisten antar entitas; perubahan struktur `model.KomponenGaji` di masa depan bisa langsung mengubah response tanpa lapisan pemetaan eksplisit.
- **Rekomendasi:** Tambah `komponenGajiResponse` + `newKomponenGajiResponse`, konsisten dengan pola `karyawan_handler.go`.

### Temuan #3 — Severity: Low
- **Kategori:** Konsistensi Struct/DTO
- **Lokasi:** `departemen_handler.go`
- **Temuan:** `Update`/`Delete` hanya mengembalikan `{"message": "..."}`, tidak mengembalikan data hasil — berbeda dari `Karyawan.Update`. (`Create`/`GetByID`/`GetAll` mengembalikan `model.Departemen` mentah, tidak masuk kategori masalah karena struktur model sederhana dan memang idealnya ditampilkan apa adanya.)
- **Dampak:** Client tidak tahu nilai final tanpa melakukan GET ulang setelah `Update`.
- **Rekomendasi:** Kembalikan `model.Departemen` (atau DTO) hasil update, konsisten dengan `Karyawan.Update`.

### Temuan #4 — Severity: Low
**Kategori:** Error Handling
**Lokasi:** `payroll_repository.go`
**Temuan:** Pesan `ErrPayrollAlreadyExists` memakai tanda seru ("...periode ini!"), beda gaya dari sentinel error lain yang tidak pakai tanda baca penekanan.
**Dampak:** Inkonsistensi gaya pesan error, murni kosmetik, tidak berpengaruh pada logic atau parsing.
**Rekomendasi:** Hapus tanda seru untuk konsistensi gaya pesan error.

### Temuan #5 — Severity: Medium
- **Kategori:** Error Handling / Validasi
- **Lokasi:** `karyawan_handler.go`, `komponen_gaji_handler.go`
- **Temuan:** `binding:"required"` pada field `decimal.Decimal` (`GajiPokok`, `Nominal`) belum diverifikasi eksplisit apakah menolak nilai `0`.
- **Dampak:** Berbeda dari Temuan #1–#4, ini bukan murni kosmetik — kalau validator ternyata meloloskan `decimal.Decimal` zero value sebagai "kosong terisi", ada risiko data gaji_pokok/nominal 0 masuk tanpa tertangkap validasi, berpotensi jadi bug fungsional nyata (bukan hanya gaya kode).
- **Rekomendasi:** Tambah test case eksplisit kirim nilai `0` untuk memastikan perilaku validator terhadap `decimal.Decimal` zero value.

---

## 3. Pengecualian Coding-Guideline (Disengaja)

Guideline umum project ini: setiap entitas idealnya punya CRUD simetris dan perilaku delete yang konsisten. Dua kasus berikut sengaja menyimpang — ini **bukan** temuan yang perlu diperbaiki (beda kategori dari section 2).

### Pengecualian #1 — `KomponenGajiService`/`KomponenGajiRepository` Tanpa Method `Delete`
- **Guideline yang dilanggar:** CRUD lengkap per entitas.
- **Alasan:** Komponen gaji yang sudah pernah dipakai dalam generate payroll historis (`payroll.total_tunjangan`/`total_potongan` adalah snapshot hasil kalkulasi darinya) tidak boleh hilang jejaknya. Jika HR salah input nominal/persentase, koreksi dilakukan lewat `Update`, bukan hapus-lalu-buat-ulang, supaya data tetap bisa ditelusuri.
- **Catatan implementasi:** Kode `Delete` yang di-comment-out di `komponen_gaji_repository.go` sengaja dibiarkan sebagai dokumentasi keputusan ini, bukan dihapus total.

### Pengecualian #2 — Hard-Delete Departemen vs Soft-Delete Karyawan
- **Guideline yang dilanggar:** Pola delete konsisten di semua entitas.
- **Alasan:** Departemen adalah master data organisasi murni tanpa histori transaksional yang melekat langsung padanya, sehingga aman dihapus permanen (dilindungi FK constraint jika masih direferensikan — lihat Bug #1 di `debugging-log.md`). Karyawan terhubung ke riwayat `payroll` yang harus tetap bisa ditelusuri untuk audit, sehingga hanya dinonaktifkan (`status = 'nonaktif'`), tidak pernah dihapus fisik.

---

## 4. Kesimpulan

Self-review mencakup file yang disebutkan di scope (model → repository → service → handler), **belum** mencakup `router.go`/`main.go`. Ditemukan 2 pengecualian desain disengaja (section 3) dan 5 temuan inkonsistensi implementasi (section 2) — 4 kosmetik (Low), 1 berpotensi fungsional (Medium, Temuan #5). Pada file yang direview: tidak ditemukan celah SQL injection, kebocoran pesan error mentah ke client (diverifikasi lewat Bug #1–#5 di `debugging-log.md`), atau pelanggaran dependency flow `handler → service → repository`.
