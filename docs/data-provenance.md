# Data Provenance — Dataset Sistem Informasi Penggajian

Dokumen ini mencatat proses analisis, evaluasi, dan transformasi dataset yang digunakan
untuk mengisi `seed.sql`. Ditulis sebagai bukti proses kerja analis terhadap instruksi
tugas ("siapkan dataset penggajian, dapat menggunakan data publik ataupun data privat")
dan arahan asesor untuk mempelajari sifat-sifat data publik vs privat sebelum digunakan.

---

## 1. Sumber Data

**Nama dataset:** Employee Salary Dataset
**Sumber:** Kaggle — [prince7489/employee-salary-dataset](https://www.kaggle.com/datasets/prince7489/employee-salary-dataset)
**Sifat:** Data publik
**Jumlah baris:** 50
**Kolom asli:** `EmployeeID`, `Name`, `Department`, `Experience_Years`, `Education_Level`, `Age`, `Gender`, `City`, `Monthly_Salary`

### 1.1 Riwayat Pemilihan Dataset

Dataset pertama yang dievaluasi ([anninasimon/employee-salary-dataset](https://www.kaggle.com/datasets/anninasimon/employee-salary-dataset), 35 baris) **ditolak** setelah profiling awal, dengan alasan:
- Hanya memiliki 5 kolom (`ID`, `Experience_Years`, `Age`, `Gender`, `Salary`) — tidak ada field departemen, jabatan, atau nama, sehingga lebih dari 80% data hasil akhir tetap akan berupa data sintetis murni. Ini secara substansi tidak berbeda dari pendekatan "data dummy" yang justru ingin dihindari.
- Kualitas data buruk: rentang `Salary` 5.000–6.000.050 (rasio ~1200x) tanpa keterangan satuan/mata uang yang jelas, indikasi kuat data noisy/tidak konsisten.
- Ditemukan baris dengan `Age` 17 dan `Experience_Years` 1 — anomali usia kerja yang perlu ditandai, bukan langsung dipakai.

Dataset kedua (yang dipakai final) dipilih karena memiliki struktur yang jauh lebih dekat dengan kebutuhan skema: mengandung `Department` (kategorikal, langsung relevan untuk tabel `departemen`) dan `Experience_Years` + `Monthly_Salary` yang bisa dipetakan ke logika jabatan dan gaji secara defensible.

---

## 2. Profiling Data Sumber

| Kolom | Tipe | Range / Nilai Unik |
|---|---|---|
| `Department` | Kategorikal | Finance, HR, IT, Marketing, Operations (5 kategori) |
| `Experience_Years` | Numerik | 1 – 19 tahun |
| `Monthly_Salary` | Numerik | 28.420 – 149.123 (satuan asli: INR, berdasarkan konteks dataset) |
| `Name` | Teks | Generik ("Employee_1", dst) — bukan nama asli |
| `Education_Level`, `Age`, `Gender`, `City` | Kategorikal/Numerik | Tidak dievaluasi lebih lanjut — lihat section 3 |

Tidak ditemukan missing value pada kolom yang dipakai (`Department`, `Experience_Years`, `Monthly_Salary`).

---

## 3. Evaluasi Kesesuaian Field terhadap Skema

Skema `karyawan` (lihat project-design.md section 2.1) membutuhkan: `nip`, `nama`, `departemen_id`, `jabatan`, `gaji_pokok`, `tanggal_masuk`, `status`.

| Field Dataset | Dipakai? | Alasan |
|---|---|---|
| `Department` | ✅ Ya | Dipetakan langsung ke `departemen.nama`, kategorikal dan konsisten (5 nilai, tidak ada varian penulisan). |
| `Experience_Years` | ✅ Ya | Basis objektif untuk menentukan `jabatan` — field numerik dengan range wajar (1–19), tidak ada anomali. |
| `Monthly_Salary` | ✅ Ya (ditransformasi) | Basis untuk `gaji_pokok`, tapi **nilai absolut tidak dipakai langsung** — lihat section 4. |
| `Name` | ❌ Tidak | Nilai generik ("Employee_1"), tidak merepresentasikan nama sungguhan. Diganti nama Indonesia sintetis. |
| `Education_Level` | ❌ Tidak | Tidak ada kolom padanan di skema; sistem penggajian ini tidak memiliki logika berbasis pendidikan. |
| `Age` | ❌ Tidak | Tidak ada kolom padanan di skema; tidak ada aturan gaji/jabatan berbasis usia dalam desain sistem. |
| `Gender` | ❌ Tidak | Tidak ada kolom padanan di skema; di luar scope sistem penggajian ini. |
| `City` | ❌ Tidak | Tidak ada kolom padanan di skema; sistem tidak membedakan lokasi kerja. |
| `nip` | — | Tidak ada di dataset sumber. 100% sintetis, dibentuk dari kombinasi tahun masuk (turunan `Experience_Years`) + nomor urut. |
| `tanggal_masuk` | — | Tidak ada di dataset sumber. Diturunkan dari `Experience_Years` (tanggal hari ini dikurangi N tahun, ditambah offset acak 0–300 hari) sebagai proksi realistis tanggal masuk kerja. |
| `status` | — | Tidak ada di dataset sumber. Default `aktif`, dengan 2 baris sengaja diset `nonaktif` sebagai edge case pengujian. |

**Kesimpulan:** dari 9 kolom dataset sumber, 3 dipakai (33%), 5 tidak relevan dengan skema, dan field sisanya (nip, tanggal_masuk, status) sepenuhnya sintetis karena memang tidak tersedia di data publik manapun untuk kasus internal seperti nomor induk pegawai.

---

## 4. Transformasi `Monthly_Salary` → `gaji_pokok`

**Keputusan: percentile remap, bukan konversi kurs.**

Dua opsi dipertimbangkan:

1. **Konversi kurs langsung** (₹ × ~190 ≈ Rp): ditolak. Nilai asli dataset (28.420–149.123 INR) dikonversi akan menghasilkan Rp5.400.000–Rp28.300.000 — kelihatan masuk akal secara kebetulan, tapi ini **konversi tanpa makna ekonomi nyata**: dataset tidak menyatakan apakah angka ini representatif untuk standar hidup/kompensasi India secara riil, dan kurs mata uang tidak relevan untuk menentukan skala gaji domestik Indonesia.
2. **Percentile rank remap** (dipakai): setiap nilai `Monthly_Salary` dikonversi jadi peringkat persentil (0.0–1.0) relatif terhadap 50 nilai lain dalam dataset, lalu di-remap linear ke rentang gaji pokok Indonesia yang ditetapkan berdasar estimasi umum (Rp5.000.000–Rp35.000.000, mencakup level Staff hingga Senior Manager).

Alasan memilih opsi 2: yang dipertahankan dari dataset sumber bukan nilai absolutnya (yang tidak applicable ke konteks Indonesia), melainkan **struktur/distribusi relatif** — siapa berpenghasilan tinggi vs rendah dibanding rekan lain, sehingga variasi data uji tetap realistis dan tidak seragam.

Formula: `gaji_pokok = 5.000.000 + percentile_rank × (35.000.000 - 5.000.000)`, dibulatkan ke kelipatan Rp50.000 terdekat.

---

## 5. Sifat Data Privat (Untuk Referensi)

Sebagai perbandingan, apabila proyek ini menggunakan data privat (misal data internal perusahaan riil), karakteristik yang akan berbeda dari pendekatan di atas:
- Field seperti `nip`, `nama`, `tanggal_masuk` akan tersedia langsung dan akurat, tidak perlu disintesis.
- Struktur jabatan dan komponen gaji akan mengikuti kebijakan HR aktual perusahaan (grade, golongan), bukan tier hasil asumsi analis berdasarkan `Experience_Years`.
- Data privat membawa isu kerahasiaan/PII yang harus ditangani (anonimisasi/masking) — tidak relevan untuk data publik seperti dataset ini yang sudah tidak memuat PII asli (`Name` sudah generik).

Dataset publik dipilih untuk proyek ini karena: (1) sesuai instruksi tugas yang mengizinkan data publik, (2) tidak ada batasan kerahasiaan sehingga dapat disertakan langsung dalam repository, (3) cukup representatif untuk mendemonstrasikan proses analisis dan transformasi data — yang justru menjadi bukti kompetensi analis, bukan sekadar bukti "punya data".

---

## 6. Edge Case yang Sengaja Disisipkan

| Edge Case | Lokasi | Tujuan Pengujian |
|---|---|---|
| 2 karyawan berstatus `nonaktif` | `karyawan` id 7, 33 | Uji filter/exclude soft-delete pada endpoint list & laporan |
| 1 karyawan tanpa `komponen_gaji` sama sekali | `karyawan` id 15 | Uji kalkulasi payroll saat tidak ada tunjangan/potongan (total = gaji pokok) |

Edge case ini melengkapi variasi data uji normal dari 50 baris dataset, mendukung skenario pengujian unit maupun integrasi di `tests/unit/` dan `tests/integration/`.
