# Lembar Pengujian Unit — Sistem Informasi Penggajian

- **Kode Test:** `tests/unit/payroll_service_test.go`, `tests/unit/sort_test.go`
- **Metode:** Table-driven test, Go `testing` package, mock repository (tanpa koneksi database asli)
- **Command Eksekusi:** `go test ./tests/unit/... -v`
- **Tanggal Eksekusi Terakhir:** 25 July 2026
- **Hasil Keseluruhan:** 18/18 skenario PASS

---

## A. Modul: `PayrollService.GeneratePayroll` (`payroll_service_test.go`)

Seluruh skenario menggunakan mock untuk `KaryawanRepository`, `KomponenGajiRepository`, `PayrollRepository`, dan `pgx.Tx` — tidak ada query ke database asli. Periode uji tetap: `2026-07-01`.

| No | Skenario | Data Uji | Expected | Actual | Status |
|---|---|---|---|---|---|
| 1 | Normal case: kombinasi tunjangan & potongan, flat & persen sekaligus | gaji_pokok=5.000.000; tunjangan flat "Transport"=500.000; tunjangan persen "Jabatan"=10%; potongan flat "BPJS Kesehatan"=200.000; potongan persen "BPJS Ketenagakerjaan"=2% | total_tunjangan=1.000.000; total_potongan=300.000; gaji_bersih=5.700.000; Create dipanggil dengan karyawan_id & periode benar; Commit dipanggil, Rollback tidak | Sesuai expected | PASS |
| 2 | Gaji pokok nol: komponen persen ikut nol, komponen flat tetap berjalan | gaji_pokok=0; tunjangan persen "Jabatan"=10%; potongan flat "BPJS Kesehatan"=200.000 | total_tunjangan=0; total_potongan=200.000; gaji_bersih=-200.000; Commit dipanggil | Sesuai expected | PASS |
| 3 | Karyawan tanpa komponen_gaji sama sekali | gaji_pokok=3.000.000; komponen_gaji=[] (slice kosong) | total_tunjangan=0; total_potongan=0; gaji_bersih=3.000.000 (= gaji_pokok apa adanya); Commit dipanggil | Sesuai expected | PASS |
| 4 | is_persen=true murni: hanya satu tunjangan persen | gaji_pokok=1.000.000; tunjangan persen "Jabatan"=10% | total_tunjangan=100.000; total_potongan=0; gaji_bersih=1.100.000 | Sesuai expected | PASS |
| 5 | is_persen=true dengan nominal 0%: kontribusi harus nol, bukan galat/error | gaji_pokok=1.000.000; tunjangan persen "Insentif Kondisional"=0% | total_tunjangan=0; total_potongan=0; gaji_bersih=1.000.000; tidak ada error/panic | Sesuai expected | PASS |
| 6 | Karyawan tidak ditemukan: harus short-circuit sebelum transaksi dibuka | `KaryawanRepository.GetByID` mengembalikan `ErrKaryawanNotFound` | error = `ErrKaryawanNotFound`; `PayrollRepository.Create` TIDAK dipanggil; `BeginTx` TIDAK dipanggil | Sesuai expected | PASS |
| 7 | Karyawan berstatus nonaktif: harus short-circuit sebelum transaksi dibuka | karyawan valid dengan status="nonaktif" | error = `ErrKaryawanTidakAktif`; `PayrollRepository.Create` TIDAK dipanggil; `BeginTx` TIDAK dipanggil | Sesuai expected | PASS |
| 8 | Repository komponen_gaji gagal: harus short-circuit sebelum transaksi dibuka | karyawan valid (gaji_pokok=5.000.000); `KomponenGajiRepository.GetByKaryawanID` mengembalikan error koneksi | error diteruskan apa adanya; `Create` TIDAK dipanggil; `BeginTx` TIDAK dipanggil | Sesuai expected | PASS |
| 9 | Payroll duplikat: sudah pernah digenerate untuk kombinasi karyawan+periode yang sama | karyawan valid (gaji_pokok=4.000.000); komponen_gaji=[]; `Create` mengembalikan `ErrPayrollAlreadyExists` | error = `ErrPayrollAlreadyExists`; `Create` DIPANGGIL (transaksi sempat dibuka); `Rollback` dipanggil; `Commit` TIDAK dipanggil | Sesuai expected | PASS |

---

## B. Modul: `util.SortKomponenGajiByNominalDesc` (`sort_test.go`)

Algoritma: insertion sort manual, in-place, stabil, descending berdasarkan `Nominal` mentah. Kompleksitas: O(n²) worst/average, O(n) best case, O(1) ruang tambahan.

| No | Skenario | Data Uji | Expected | Actual | Status |
|---|---|---|---|---|---|
| 9 | Data acak: harus terurut descending | Nominal input: [500.000, 100.000, 750.000, 250.000] | Output: [750.000, 500.000, 250.000, 100.000] | Sesuai expected | PASS |
| 10 | Data sudah terurut descending (best case): urutan tidak boleh berubah | Nominal input: [900.000, 500.000, 100.000] | Output tetap: [900.000, 500.000, 100.000] | Sesuai expected | PASS |
| 11 | Data terurut ascending (worst case): harus dibalik total | Nominal input: [100.000, 500.000, 900.000] | Output: [900.000, 500.000, 100.000] | Sesuai expected | PASS |
| 12 | Slice kosong: tidak boleh panic | input = [] (slice kosong) | Output = [] tanpa panic/error | Sesuai expected | PASS |
| 13 | Satu elemen: tidak berubah | Nominal input: [42.000] | Output: [42.000] | Sesuai expected | PASS |
| 14 | Semua nominal sama: tidak boleh panic, urutan tetap valid | Nominal input: [100.000, 100.000, 100.000] | Output: [100.000, 100.000, 100.000] tanpa panic | Sesuai expected | PASS |
| 15 | Melibatkan nominal nol (edge case is_persen=true dengan nominal 0%) | Nominal input: [0, 500.000, 0] | Output: [500.000, 0, 0] | Sesuai expected | PASS |
| 16 | Stabilitas: elemen ber-nominal sama harus mempertahankan urutan relatif asli | ID 10,20,30 semua Nominal=100.000, diselingi ID 99 Nominal=999.000 | Setelah sort: ID 99 di depan (999.000), lalu ID 10, 20, 30 berurutan (urutan asli tidak tertukar) | Sesuai expected | PASS |
| 17 | In-place: slice asli harus ikut termodifikasi, bukan mengembalikan copy baru | input = [(ID1,100.000), (ID2,900.000)]; slice header sama dipakai sebagai referensi `original` | `original[0].Nominal` = 900.000 setelah sort (backing array yang sama ikut berubah) | Sesuai expected | PASS |

---

## Catatan Metodologi

- Semua mock (`mockKaryawanRepo`, `mockKomponenGajiRepo`, `mockPayrollRepo`, `mockTx`) meng-embed interface aslinya (nilai nil) dan hanya meng-override method yang benar-benar dipanggil oleh `PayrollService`. Jika ada method lain yang tak sengaja terpanggil, test akan panic (nil pointer) — ini disengaja sebagai sinyal bahwa mock perlu diperbarui, bukan silently pass.
- Skenario 6-8 memverifikasi bukan hanya nilai error, tapi juga efek samping (apakah `Create`/`BeginTx`/`Commit`/`Rollback` dipanggil atau tidak) — memastikan transaksi pgx eksplisit (lihat `docs/debugging-log.md` Bug #11) benar-benar berfungsi sesuai desain, bukan hanya lolos compile.
- Skenario 16-17 di atas ditulis sebagai test terpisah (`TestSortKomponenGajiByNominalDesc_Stable`, `TestSortKomponenGajiByNominalDesc_InPlace`) karena menguji properti algoritma yang berbeda dari korektnes urutan semata.
- Dokumen ini adalah bukti formal terpisah dari kode test. Kode test yang menjadi rujukan data uji: `tests/unit/payroll_service_test.go` dan `tests/unit/sort_test.go`.
