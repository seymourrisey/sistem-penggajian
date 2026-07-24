# Analisis Skalabilitas — Sistem Informasi Penggajian

## 1. Lingkup Sistem & Lingkungan Operasi

**Lingkup data riil:** Dataset seed (`migrations/seed.sql`) berisi **50 baris karyawan**, diambil dan ditransformasi dari dataset publik Kaggle *Employee Salary Dataset* (lihat `ProjectDesign-SistemPenggajian.md` section 4.3). Ini adalah skala data aktual proyek untuk keperluan uji kompetensi — bukan skala production riil.

**Skenario stress-test:** Untuk membuktikan pemahaman terhadap *pola* skalabilitas (bukan sekadar klaim teoretis), pengujian performa (`docs/profiling-report.md`) dilakukan pada database terpisah (`payroll_profiling_db`) dengan data sintetis yang diperbesar ke **5.000 karyawan / 15.000 baris payroll (3 periode)**. Ini proyeksi disengaja untuk mensimulasikan pertumbuhan data ~100x dari seed asli, agar analisis index dan bottleneck punya dasar empiris, bukan asumsi kosong.

**Lingkungan operasi:** Target deployment production **belum ditentukan** — proyek ini bersifat studi kasus/uji kompetensi, bukan sistem yang sudah punya kontrak infrastruktur nyata. Analisis kebutuhan perangkat keras di bagian 3 karena itu bersifat **estimasi hipotetis** berbasis pola beban yang teramati saat profiling (dilakukan di laptop dev: Intel i7-7600U @ 2.80GHz, Windows), bukan rekomendasi final untuk server tertentu.

---

## 2. Analisis Kompleksitas vs Jumlah Data

Dua pola berbeda ditemukan tergantung selektivitas filter query (bukti: `profiling-report.md` §1):

| Pola Query | Contoh Endpoint | Selektivitas | Perilaku saat Data Bertambah |
|---|---|---|---|
| Filter longgar (banyak baris cocok) | `GET /api/payroll/laporan` | ~33% dari tabel `payroll` | Seq scan tetap lebih murah dari index scan (cost-based planner) — kompleksitas efektif ~O(n) terhadap ukuran tabel, index tidak membantu |
| Filter ketat (sedikit baris cocok) | `GET /api/payroll/:id/riwayat` | ~0.02% dari tabel `payroll` | Index scan menang telak (~24x lebih cepat) — kompleksitas efektif ~O(log n) via B-tree, **manfaat index makin besar seiring tabel membesar** |

**Implikasi untuk pertumbuhan data:** Endpoint riwayat (per-karyawan) akan tetap cepat berapa pun jumlah karyawan bertambah, karena filternya selalu tunggal. Endpoint laporan (agregat per periode) akan makin lambat seiring jumlah baris payroll per periode bertambah (linear terhadap jumlah karyawan aktif) — ini yang paling perlu dipantau saat data riil tumbuh jauh melebihi 50 baris seed.

**Algoritma sorting manual** (`internal/util/sort.go`) terbukti O(n²) empiris (`profiling-report.md` §3), namun dampaknya diabaikan pada skala realistis sistem ini (komponen gaji per karyawan biasanya belasan baris, bukan ribuan) — tidak jadi perhatian skalabilitas prioritas.

---

## 3. Kebutuhan Perangkat Keras (Estimasi Hipotetis)

Karena target deployment belum ditentukan (bagian 1), estimasi berikut berbasis pola beban yang teramati, bukan sizing formal:

- **CPU:** Beban dominan saat load test bukan komputasi query/algoritma, melainkan I/O logging (`runtime.cgocall` 36–46% samples, `profiling-report.md` §4) — pada mode production dengan logging dinonaktifkan, kebutuhan CPU jauh lebih rendah dari yang teramati di profiling. Untuk skala seed (50 karyawan), 1-2 vCPU cukup.
- **RAM:** Total alokasi heap per 20 detik load test paralel (5 job) berkisar 62–293 MB (`profiling-report.md` §2), dengan `inuse_space` setelah GC hanya ~3.5-4.1 MB — jejak memory aktif kecil. 1-2 GB RAM cukup untuk skala aplikasi ini plus overhead PostgreSQL.
- **Storage:** Minimal, karena tidak ada penyimpanan file/gambar; kebutuhan storage didominasi oleh pertumbuhan tabel `payroll` (snapshot per periode per karyawan) dan `komponen_gaji`.

---

## 4. Index Strategy

Tiga index custom (`idx_karyawan_departemen`, `idx_payroll_karyawan_periode`, `idx_komponen_karyawan`) dipasang di kolom yang dipakai `WHERE`/`JOIN` (`ProjectDesign-SistemPenggajian.md` §2.2), bukan dipasang di semua kolom secara serampangan. Bukti empiris (`profiling-report.md` §1) menunjukkan:

- **`idx_payroll_karyawan_periode`** tidak memberi percepatan terukur pada volume/pola query saat ini (selektivitas ~33%) — tetap dipertahankan sebagai antisipasi jangka panjang (bila jumlah periode bertambah signifikan, mis. data 5 tahun = 60 periode, selektivitas filter periode akan jauh lebih tajam).
- **`payroll_karyawan_id_periode_key`** (index tersembunyi dari `UNIQUE(karyawan_id, periode)`) terbukti memberi percepatan ~24x pada query selektivitas tinggi — index ini berfungsi ganda: enforcement business rule (cegah duplikasi payroll) sekaligus index performa.

**Prinsip yang dipegang:** efektivitas index bergantung pada selektivitas filter, bukan keberadaan index itu sendiri — tidak semua kolom otomatis butuh index, keputusan index diverifikasi lewat `EXPLAIN ANALYZE`, bukan asumsi.

---

## 5. Connection Pooling

Konfigurasi aktual (`pkg/database/postgres.go`):

| Parameter | Nilai | Alasan |
|---|---|---|
| MaxConns | 10 | Cukup untuk beban paralel skala seed (50 karyawan) + load test profiling (5 job paralel) tanpa menghabiskan connection slot PostgreSQL |
| MinConns | 2 | Menjaga koneksi siap pakai tanpa overhead cold-start di setiap request |
| MaxConnLifetime | 1 jam | Mencegah koneksi basi/stale bertahan terlalu lama |
| MaxConnIdleTime | 30 menit | Melepas koneksi idle untuk efisiensi resource saat beban rendah |
| HealthCheckPeriod | 1 menit | Deteksi dini koneksi mati sebelum dipakai request |

Pool diverifikasi dengan `Ping()` saat startup — kegagalan koneksi terdeteksi di awal, bukan saat request pertama masuk.

**Catatan skalabilitas:** `MaxConns=10` adalah nilai awal yang wajar untuk skala seed saat ini. Jika beban riil bertambah jauh (concurrent request tinggi), ini parameter pertama yang perlu dinaikkan — bersama peningkatan `max_connections` di sisi PostgreSQL server.

---

## 6. Potensi Bottleneck

Berdasarkan profiling (`profiling-report.md` §4), pada lingkungan pengujian ini:

1. **I/O logging console** (Gin debug mode, synchronous write ke console) — bottleneck dominan di kedua endpoint yang diuji, bukan query database. Direkomendasikan nonaktif/diarahkan ke file untuk production.
2. **`math/big` (via `shopspring/decimal`)** — kontributor alokasi memory terbesar di luar kode aplikasi sendiri (~10-15% di kedua endpoint). Ini trade-off desain disengaja (presisi finansial), bukan bug, tapi tetap relevan dipantau jika volume request bertambah drastis.
3. **Algoritma sorting O(n²)** — bukan bottleneck saat ini (skala data kecil), tapi jadi risiko laten jika suatu saat jumlah komponen gaji per karyawan bertambah jauh di luar pola wajar (belasan baris).

Tidak ditemukan indikasi memory leak selama pengujian (`profiling-report.md` §2.5).

---

## 7. Opsi Scaling

**Vertikal (prioritas pertama):** Untuk skala proyek ini (seed 50 karyawan, hipotetis hingga ribuan), scaling vertikal (tambah CPU/RAM pada server yang sama) sudah cukup — arsitektur saat ini (single-server, connection pool, index selektif) belum mendekati batas yang butuh scaling horizontal.

**Horizontal (opsi masa depan, belum dibutuhkan saat ini):**
- Read replica PostgreSQL — jika endpoint laporan (read-heavy, agregasi) jadi beban signifikan terpisah dari transaksi tulis (`GeneratePayroll`).
- Load balancer + multiple instance API — jika concurrent request jauh melampaui kapasitas satu instance Gin.

Kedua opsi ini **tidak diimplementasikan** dalam scope proyek (sesuai batasan scope, `ProjectDesign-SistemPenggajian.md` §1.3) — didokumentasikan sebagai kesadaran arah scaling, bukan kebutuhan aktual saat ini.

---

## 8. Kesimpulan

Skala data riil proyek ini (50 karyawan seed) jauh di bawah titik di mana skalabilitas jadi masalah nyata. Keputusan desain (index selektif, connection pool, snapshot pattern di `payroll`) sudah mengantisipasi pertumbuhan data lewat pengujian empiris pada skala 100x lebih besar (5.000 karyawan), bukan asumsi teoretis semata. Bottleneck yang ditemukan saat ini bersifat konfigurasi environment (logging), bukan struktural — arsitektur single-server dengan connection pooling sudah memadai untuk kebutuhan proyek dan punya jalur jelas ke scaling vertikal/horizontal jika dibutuhkan di masa depan.
