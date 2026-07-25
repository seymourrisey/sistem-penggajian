# Profiling Report - Sistem Informasi Penggajian

- **Tools:** `net/http/pprof` (CPU & heap profile), `go tool pprof`, `EXPLAIN ANALYZE` (PostgreSQL), `go test -bench` (benchmark kompleksitas algoritma)
- **Database:** `payroll_profiling_db` (terisolasi dari `payroll_db`/`payroll_test_db`), data sintetis 5.000 karyawan / 15.000 payroll (3 periode)
- **Environment eksekusi:** laptop developer: Intel(R) Core(TM) i7-7600U CPU @ 2.80GHz, Windows. 
Angka absolut (ns/op, ms) bersifat kontekstual terhadap hardware ini, bukan diklaim setara server produksi - yang jadi bukti utama adalah **pola relatif** (rasio before/after, rasio pertumbuhan terhadap N), yang tidak bergantung pada hardware spesifik.

Laporan ini mencakup 3 dimensi profiling: **waktu eksekusi** (CPU profile, 2 kasus dengan selektivitas berbeda), **penggunaan memory** (heap profile, 2 endpoint yang sama), dan **kompleksitas algoritma** (benchmark empiris terhadap sorting manual).

---

## 1. CPU Profiling - Waktu Eksekusi

### 1.1 Kasus 1 — `GET /api/payroll/laporan?periode=2026-07-01` (selektivitas rendah, ~33%)

**Metodologi:**
1. `DROP INDEX` ketiga index custom (`idx_karyawan_departemen`, `idx_payroll_karyawan_periode`, `idx_komponen_karyawan`) di `payroll_profiling_db`.
2. Capture CPU profile 30 detik dengan beban paralel (5 job background hit endpoint terus-menerus) → `profile-tanpa-index.pb.gz`.
3. `CREATE INDEX` ulang, capture identik → `profile-dengan-index.pb.gz`.
4. `EXPLAIN ANALYZE` manual di psql, dengan index terpasang.

**Hasil pprof:**

| Metrik | Tanpa Index | Dengan Index |
|---|---|---|
| Duration capture | 30s | 30s |
| Total samples | 6.67s (22.23%) | 7.11s (23.70%) |
| `GetLaporanAgregat` — cumulative | 1.96s (29.39%) | 2.14s (30.10%) |

**Temuan: tidak ada perbaikan performa yang terukur.** Angka dengan index justru sedikit lebih tinggi, selisih ini berada dalam rentang wajar variasi antar-run, bukan efek index memperlambat.

**EXPLAIN ANALYZE:**

```sql
EXPLAIN ANALYZE
SELECT d.nama, p.periode, COUNT(p.id), SUM(p.gaji_bersih), AVG(p.gaji_bersih)
FROM payroll p
JOIN karyawan k ON p.karyawan_id = k.id
JOIN departemen d ON k.departemen_id = d.id
WHERE p.periode = '2026-07-01'
GROUP BY d.nama, p.periode;
```

```
HashAggregate  (actual time=4.372..4.377 rows=8)
  ->  Hash Join  (actual time=0.890..3.542 rows=5000)
        ->  Hash Join  (actual time=0.867..2.896 rows=5000)
              ->  Seq Scan on payroll p  (actual time=0.008..1.134 rows=5000)
                    Filter: (periode = '2026-07-01'::date)
                    Rows Removed by Filter: 10000
              ->  Hash
                    ->  Seq Scan on karyawan k  (actual time=0.007..0.468 rows=5000)
        ->  Hash
              ->  Seq Scan on departemen d  (actual time=0.012..0.013 rows=8)
Planning Time: 4.345 ms
Execution Time: 4.486 ms
```

**Planner tidak memakai `idx_payroll_karyawan_periode` sama sekali** — keputusan cost-based yang benar. PostgreSQL tidak memakai ambang selektivitas tetap; keputusan seq scan vs index scan dihitung ulang tiap query dari estimasi cost (page cost, row cost, statistik tabel). Untuk query ini, dengan 33% baris `payroll` cocok filter (5.000 dari 15.000), biaya membaca hampir semua halaman tabel secara sequential lebih murah daripada random I/O per baris lewat index, konsisten dengan heuristik umum bahwa index scan biasanya kalah bersaing begitu porsi baris yang cocok filter cukup besar (bukan aturan baku, melainkan hasil kalkulasi biaya). Estimasi planner (rows=5000) sama persis dengan aktual, statistik tabel akurat, bukan basi.

**Kesimpulan:** Index tidak memberi percepatan terukur pada volume & pola query ini. Dipertahankan di schema sebagai antisipasi skalabilitas jangka panjang (lihat NF2, `project-design.md` section 2.2) — jika jumlah periode bertambah signifikan (mis. data 5 tahun = 60 periode), selektivitas filter periode akan jauh lebih tajam.

### 1.2 Kasus 2 - `GET /api/payroll/:karyawan_id/riwayat` (selektivitas tinggi, ~0.02%)

Dipilih sebagai pembanding langsung: filter `karyawan_id` tunggal dari 5.000 karyawan (3 baris dari 15.000 total) — skenario tekstbuku di mana index scan seharusnya menang telak, kontras dengan Kasus 1.

**Catatan koreksi metodologi (penting, ditemukan di tengah proses):** Percobaan pertama "tanpa index" (`DROP INDEX idx_payroll_karyawan_periode` saja) **tidak valid** — tabel `payroll` juga punya `UNIQUE(karyawan_id, periode)` di schema, dan PostgreSQL otomatis membuat index tersembunyi (`payroll_karyawan_id_periode_key`) untuk constraint tersebut, terlepas dari index eksplisit yang di-drop. `EXPLAIN ANALYZE` capture pertama masih menunjukkan `Index Scan using payroll_karyawan_id_periode_key`. Setelah disadari, constraint tersebut juga di-drop sementara (khusus di `payroll_profiling_db`, direstore setelah capture selesai — diverifikasi via `\d payroll` identik dengan schema asli) untuk mendapat kondisi tanpa-index yang benar.

**EXPLAIN ANALYZE — Bukti Utama (`karyawan_id = 4790`):**

| Kondisi | Query Plan | Execution Time |
|---|---|---|
| Dengan index (`payroll_karyawan_id_periode_key`) | `Index Scan`, `Index Cond: (karyawan_id = 4790)`, Buffers: shared hit=3 | **0.041 ms** |
| Tanpa index (constraint & index custom di-drop) | `Seq Scan`, `Rows Removed by Filter: 14997`, Buffers: shared hit=170 | **0.981 ms** |

**~24x lebih cepat dengan index.** Bukti kuantitatif utama KUK "menunjukkan peningkatan performa rancangan metode" — bersih dan deterministik.

**pprof — dicatat dengan caveat jujur:**

| Metrik | Dengan Index | Tanpa Index |
|---|---|---|
| Total samples (10s window) | 15.13s (151.30%) | 7.37s (73.70%) |
| `GetRiwayatByKaryawanID` — cumulative | 3.48s (23.00%) | 1.50s (20.35%) |

Total samples kedua capture berbeda jauh — mengindikasikan variance throughput antar-run (beban background OS, GC timing), bukan variabel terkontrol seperti `EXPLAIN ANALYZE`. **Angka cumulative pprof ini sengaja tidak dijadikan klaim kuantitatif utama** karena confounded; `EXPLAIN ANALYZE` di atas yang jadi bukti performa. pprof tetap berguna sebagai konteks: di kedua kondisi, porsi terbesar sample tetap didominasi `runtime.cgocall` dan I/O logging console (36–46%), konsisten dengan Kasus 1.

**Kesimpulan:** Index `payroll_karyawan_id_periode_key` memberi peningkatan performa nyata (~24x execution time) untuk query selektivitas tinggi — menegaskan Kasus 1 dari sisi berlawanan: efektivitas index bergantung pada selektivitas filter, bukan keberadaan index itu sendiri.

---

## 2. Memory Profiling - Alokasi & Potensi Leak

### 2.1 Metodologi & Catatan Koreksi

Heap profile (`go tool pprof http://localhost:8080/debug/pprof/heap`) dicapture untuk kedua endpoint (laporan & riwayat), masing-masing dengan beban paralel 5 job selama 20 detik.

**Koreksi penting:** `alloc_space` bersifat **kumulatif sejak proses `main.exe` pertama start**, bukan snapshot per-window seperti CPU profile. Capture pertama untuk endpoint laporan sempat tercemar data dari load test endpoint riwayat sebelumnya (app belum di-restart) — top allocator keliru menunjukkan `GetRiwayatByKaryawanID` padahal beban yang dijalankan cuma endpoint laporan. **Fix:** app di-restart fresh (proses lama dipastikan mati via `Get-Process`) sebelum setiap capture heap, satu endpoint per proses, tidak dicampur.

### 2.2 Kasus 1 — Endpoint Laporan

| Metrik | Nilai |
|---|---|
| Total alloc_space | 62.04 MB |
| `GetLaporanAgregat` — cumulative alloc | 28.01 MB (45.14%) |
| Total inuse_space (setelah GC) | 4115.52 kB |
| Alokasi tersisa di fungsi bisnis | 512.14 kB — proporsional, **tidak ditemukan indikasi leak** |

### 2.3 Kasus 2 — Endpoint Riwayat

| Metrik | Nilai |
|---|---|
| Total alloc_space | 293.09 MB |
| `GetRiwayatByKaryawanID` — cumulative alloc | 137.02 MB (46.75%) |
| `karyawanRepository.GetByID` (validasi existence) — cumulative alloc | 35.01 MB (11.94%) |
| Total inuse_space (setelah GC) | 3593.80 kB |
| Alokasi tersisa di fungsi bisnis | 512.14 kB — proporsional, **tidak ditemukan indikasi leak** |

Total alloc_space riwayat jauh lebih besar dari laporan (293MB vs 62MB) — bukan indikasi boros per-request, melainkan throughput lebih tinggi dalam window waktu yang sama (query lebih cepat karena index → lebih banyak request selesai diproses → lebih banyak alokasi kumulatif).

### 2.4 Modul Teridentifikasi Bermasalah (dari Sisi Alokasi Memory)

**`math/big`** (dipakai internal oleh `shopspring/decimal` untuk presisi desimal) konsisten muncul di **kedua** endpoint — bukan kebetulan sekali capture: 14.51% (9MB) di endpoint laporan (`math/big.nat.make`), ~10-15% di endpoint riwayat (kombinasi `math/big.(*Int).Text`, `math/big.nat.make`, `math/big.nat.itoa`, terkait konversi `decimal.Decimal`).

Ini **trade-off desain yang disengaja** (presisi finansial vs memory footprint), bukan bug — `decimal.Decimal` dipilih justru untuk menghindari floating-point error di kalkulasi gaji (lihat `project-design.md` section 2.2).

### 2.5 Kesimpulan Memory

Tidak ditemukan indikasi memory leak di kedua endpoint selama pengujian — `inuse_space` kecil di keduanya, tidak ada fungsi bisnis yang menahan memory berlebih setelah garbage collection. `math/big`/`shopspring/decimal` adalah kontributor alokasi terbesar di luar kode aplikasi sendiri, konsisten dengan trade-off presisi finansial yang disengaja.

---

## 3. Kompleksitas Algoritma - Sorting Manual (`internal/util/sort.go`)

### 3.1 Metodologi

`SortKomponenGajiByNominalDesc` (insertion sort manual, F7) diklaim di komentar kode: O(n²) worst/average case, O(n) best case, O(1) ruang auxiliary. Untuk membuktikan klaim ini secara empiris (bukan cuma teoretis), dibuat benchmark (`tests/unit/sort_bench_test.go`, `go test -bench`) yang mengukur waktu eksekusi pada 6 ukuran input (N = 10, 100, 500, 1000, 2000, 4000) untuk 3 skenario:
- **Random** (average case, realistis)
- **Worst case** (ascending — kebalikan total dari target descending)
- **Best case** (descending — sudah sesuai urutan target)

Data fresh dibuat ulang per iterasi benchmark (di luar timer) supaya tidak ada efek "sudah tersortir dari iterasi sebelumnya" yang mengaburkan pengukuran.

### 3.2 Hasil

| N | Random (ns/op) | Worst Case (ns/op) | Best Case (ns/op) |
|---|---|---|---|
| 10 | 683.6 | 965.5 | 597.1 |
| 100 | 29,537 | 53,931 | 1,752 |
| 500 | 672,599 | 1,291,655 | 7,462 |
| 1,000 | 2,618,968 | 5,160,288 | 14,276 |
| 2,000 | 10,253,325 | 20,763,574 | 27,026 |
| 4,000 | 43,588,711 | 86,176,242 | 52,585 |

`B/op` dan `allocs/op`: **0 di semua skenario** — mengonfirmasi klaim "O(1) auxiliary, in-place, tanpa alokasi baru" di komentar kode secara empiris.

**Rasio pertumbuhan ns/op saat N didobel (500→1000→2000→4000):**

| Skenario | Rasio rata-rata | Kompleksitas terkonfirmasi |
|---|---|---|
| Random | ~3.9x – 4.25x | **O(n²)** (mendekati 4x, sesuai teori) |
| Worst case | ~4.0x – 4.15x | **O(n²)** (mendekati 4x, sesuai teori) |
| Best case | ~1.89x – 1.95x | **O(n)** (mendekati 2x, sesuai teori) |

### 3.3 Dampak Nyata Kompleksitas terhadap Performa

Di N=4000, worst case (86.176.242 ns) **~1639x lebih lambat** dari best case (52.585 ns; 86.176.242 ÷ 52.585 ≈ 1638.8) — bukti konkret bahwa kompleksitas O(n²) berdampak nyata pada performa saat ukuran input besar.

**Konteks skala realistis:** Pada N=10 (mendekati skala nyata aplikasi ini, komponen gaji per karyawan biasanya belasan baris, bukan ribuan), selisih worst vs best case kecil (965.5 ns vs 597.1 ns). Ini menegaskan justifikasi desain yang sudah ada di `sort.go`: 
insertion sort dipilih karena kesederhanaan lebih relevan daripada O(n log n) di skala data endpoint ini — dampak O(n²) yang signifikan baru terlihat di ukuran input yang jauh melampaui kebutuhan aktual sistem.

---

## 4. Bottleneck & Rekomendasi

**Bottleneck dominan (kedua endpoint, Kasus 1 & 2):** overhead I/O logging console (`runtime.cgocall` 36–46% dari total CPU samples, rantai syscall `syscall.WSASend`/`internal/poll`/`os.(*File).Write`) — bukan biaya query database. Ini artefak Gin debug mode + logging synchronous ke console Windows, bukan karakteristik query.

**Rekomendasi (analisis saja, tidak diuji before/after — cukup satu demonstrasi kuantitatif untuk KUK ini, sudah dipenuhi index di Kasus 2):** untuk production, nonaktifkan Gin debug logging atau arahkan log ke file (bukan console synchronous write) untuk mengurangi overhead I/O yang mendominasi profile tapi tidak relevan dengan business logic.

---

## 5. Kesimpulan Keseluruhan

1. **Index tidak otomatis mempercepat semua query** — efektivitasnya bergantung pada selektivitas filter. Laporan (~33% selektivitas) tidak terbantu; riwayat (~0.02% selektivitas) terbantu ~24x di level execution time.
2. **Tidak ditemukan indikasi memory leak** di kedua endpoint yang diprofil; kontributor alokasi terbesar (`math/big`/`shopspring/decimal`) adalah trade-off presisi finansial yang disengaja, bukan bug.
3. **Kompleksitas O(n²) insertion sort terbukti empiris** (rasio ~4x per dobel N), berdampak nyata di N besar (~1639x lebih lambat di N=4000 worst vs best case), namun dampak praktis kecil di skala data realistis aplikasi ini (belasan baris per karyawan) — insertion sort tetap pilihan desain yang tepat untuk scope ini.
4. **Bottleneck dominan pada lingkungan pengujian ini** (bukan diklaim berlaku umum di production) **bukan di query/algoritma** — melainkan overhead I/O logging console (mode development). Direkomendasikan dinonaktifkan/diarahkan ke file untuk production.
5. Kedua index (`idx_payroll_karyawan_periode`, `payroll_karyawan_id_periode_key`) dipertahankan di schema — yang pertama sebagai antisipasi skalabilitas jangka panjang, yang kedua terbukti bermanfaat langsung sekaligus berfungsi ganda sebagai enforcement business rule (unique constraint).

---

## 6. File Pendukung

- `docs/profiling/profile-tanpa-index.pb.gz`, `docs/profiling/profile-dengan-index.pb.gz` — CPU profile endpoint laporan
- `docs/profiling/profile-riwayat-tanpa-index.pb.gz`, `docs/profiling/profile-riwayat-dengan-index.pb.gz` — CPU profile endpoint riwayat
- `docs/profiling/heap-laporan.pb.gz`, `docs/profiling/heap-riwayat.pb.gz` — heap profile kedua endpoint
- `docs/profiling/sort-benchmark-result.txt` — hasil mentah `go test -bench` untuk kompleksitas algoritma
- `tests/unit/sort_bench_test.go` — kode benchmark
- Cara membuka profile: `go tool pprof -top <file>.pb.gz` (tambahkan `-alloc_space`/`-inuse_space` untuk heap profile)
