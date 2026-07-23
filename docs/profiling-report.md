# Profiling Report — Sistem Informasi Penggajian

- **Endpoint yang diprofil:** `GET /api/payroll/laporan?periode=2026-07-01`
- **Tools:** `net/http/pprof` (CPU profile), `go tool pprof`, `EXPLAIN ANALYZE` (PostgreSQL)
- **Database:** `payroll_profiling_db` (terisolasi dari `payroll_db`/`payroll_test_db`), data sintetis 5.000 karyawan / 15.000 payroll (3 periode)
- **File profile mentah:** `docs/profiling/profile-tanpa-index.pb.gz`, `docs/profiling/profile-dengan-index.pb.gz`

---

## 1. Metodologi

Karena schema project sudah memakai index sejak awal desain (bukan ditambahkan belakangan sebagai optimasi), tidak ada kondisi "sebelum index" yang historis. Untuk tetap bisa melakukan perbandingan before/after yang jujur, dilakukan:

1. `DROP INDEX` ketiga index custom (`idx_karyawan_departemen`, `idx_payroll_karyawan_periode`, `idx_komponen_karyawan`) di `payroll_profiling_db`.
2. Capture CPU profile 30 detik dengan beban paralel (5 job background hit endpoint laporan terus-menerus) → `profile-tanpa-index.pb.gz`.
3. `CREATE INDEX` ulang (index sama seperti schema asli).
4. Ulangi capture dengan parameter identik (30 detik, 5 job paralel, endpoint sama) → `profile-dengan-index.pb.gz`.
5. Tambahan: `EXPLAIN ANALYZE` manual di psql pada query yang sama, dengan index terpasang, untuk melihat keputusan query planner secara langsung.

Operasi ini murni pada database (`payroll_profiling_db`), tidak ada perubahan kode aplikasi.

---

## 2. Hasil pprof — Before vs After

| Metrik | Tanpa Index | Dengan Index |
|---|---|---|
| Duration capture | 30s | 30s |
| Total samples | 6.67s (22.23%) | 7.11s (23.70%) |
| `GetLaporanAgregat` — cumulative | 1.96s (29.39%) | 2.14s (30.10%) |

**Temuan: tidak ada perbaikan performa yang terukur.** Angka `GetLaporanAgregat` dengan index (2.14s/30.10%) justru sedikit lebih tinggi dibanding tanpa index (1.96s/29.39%). Selisih ini berada dalam rentang wajar variasi antar-run (total samples kedua capture juga sedikit berbeda, 6.67s vs 7.11s), sehingga **tidak bisa disimpulkan index memperlambat** — yang bisa disimpulkan secara valid adalah index tidak memberi dampak percepatan yang terlihat pada beban ini.

**Catatan noise dominan (berlaku di kedua capture):** porsi terbesar sample berasal dari `runtime.cgocall` (39.80% pada capture "dengan index", 36.73% pada "tanpa index") dan rantai syscall I/O (`syscall.WSASend`, `os.(*File).Write`, `internal/poll.(*FD).Write`) — ini overhead logging debug Gin ke console Windows selama load test, **bukan** biaya query database. Karena noise ini konsisten muncul di kedua sisi, perbandingan relatif `GetLaporanAgregat` tetap sah dilakukan, tapi menjelaskan kenapa selisih 1.96s→2.14s berukuran kecil dan tidak konklusif — jauh lebih kecil dari porsi noise itu sendiri.

---

## 3. EXPLAIN ANALYZE — Bukti Level Planner

Query laporan (dengan index terpasang) dijalankan langsung di psql:

```sql
EXPLAIN ANALYZE
SELECT d.nama, p.periode, COUNT(p.id), SUM(p.gaji_bersih), AVG(p.gaji_bersih)
FROM payroll p
JOIN karyawan k ON p.karyawan_id = k.id
JOIN departemen d ON k.departemen_id = d.id
WHERE p.periode = '2026-07-01'
GROUP BY d.nama, p.periode;
```

**Hasil (ringkas):**

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

**Temuan kunci: planner sama sekali tidak memakai `idx_payroll_karyawan_periode`, meski index itu ada.** Query menggunakan seq scan penuh pada `payroll`, `karyawan`, dan `departemen`.

Ini **bukan bug** dan bukan tanda index gagal dibuat. Ini keputusan cost-based yang benar dari query planner PostgreSQL:

- Filter `periode = '2026-07-01'` menyaring 5.000 dari 15.000 baris `payroll` — selektivitas ~33%.
- Index scan hanya lebih murah dari seq scan ketika filter cukup selektif (umumnya di bawah ~5–10% baris tabel). Pada selektivitas 33%, biaya random I/O per baris hasil index scan lebih mahal dibanding satu kali sequential scan penuh + hash join.
- Estimasi planner (rows=5000 estimated) persis sama dengan hasil aktual (rows=5000 actual) — artinya statistik tabel akurat (`ANALYZE` sudah tercermin), sehingga keputusan planner ini bukan karena statistik basi, melainkan memang perhitungan biaya yang tepat untuk data & query ini.
- Execution time keseluruhan hanya **4.486 ms** — sudah sangat cepat pada volume data profiling (15.000 baris payroll), terlepas dari index dipakai atau tidak.

---

## 4. Interpretasi & Kesimpulan

1. **Index tidak memberi percepatan terukur pada query laporan di volume data saat ini (15.000 baris payroll, 3 periode).** Baik dari pprof (before/after nyaris identik) maupun EXPLAIN ANALYZE (index tidak dipakai sama sekali oleh planner).
2. **Index tetap relevan untuk skalabilitas jangka panjang, bukan untuk beban saat ini.** Karakteristik query laporan ini — filter `periode` menghasilkan seleksi ~33% dari total baris karena hanya ada 3 periode di data profiling — membuat seq scan lebih murah. Jika jumlah periode bertambah signifikan (mis. data 5 tahun = 60 periode bulanan), filter `periode` akan menyaring baris jauh lebih sedikit secara relatif, dan pada titik itu index scan akan mulai lebih murah dari seq scan. Index `idx_payroll_karyawan_periode` disiapkan untuk skenario itu (lihat NF2, `ProjectDesign-SistemPenggajian.md` section 2.2), bukan untuk skala data uji saat ini.
3. **Bottleneck aktual endpoint ini, berdasarkan pprof, bukan di query database** — execution time SQL (4.486 ms) jauh lebih kecil dari cumulative time `GetLaporanAgregat` di pprof (~2s dari window 30 detik beban paralel). Selisih ini didominasi overhead jaringan I/O dan logging console (lihat catatan noise section 2), yang merupakan artefak dari mode development (Gin debug mode + logging ke terminal Windows), bukan karakteristik query itu sendiri.
4. **Rekomendasi:** untuk production, nonaktifkan Gin debug logging / arahkan log ke file (bukan console synchronous write) untuk mengurangi overhead I/O yang tidak relevan dengan business logic. Untuk index, keputusan mempertahankan `idx_payroll_karyawan_periode` di schema tetap tepat sebagai antisipasi pertumbuhan data periode, meski tidak terlihat manfaatnya pada volume data uji saat ini — trade-off ini terdokumentasi dan disengaja, bukan index yang salah desain.

---

## 5. File Pendukung

- `docs/profiling/profile-tanpa-index.pb.gz` — capture CPU profile tanpa index (baseline)
- `docs/profiling/profile-dengan-index.pb.gz` — capture CPU profile dengan index
- Cara membuka: `go tool pprof -top docs/profiling/profile-tanpa-index.pb.gz` (atau varian `-dengan-index`)
