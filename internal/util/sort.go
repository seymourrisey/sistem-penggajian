package util

import (
	"github.com/seymourrisey/sistem-penggajian/internal/model"
)

// SortKomponenGajiByNominalDesc mengurutkan slice komponen_gaji berdasarkan
// field Nominal, dari terbesar ke terkecil, menggunakan insertion sort
// manual — bukti KUK unit kompetensi #4 ("membuat algoritma untuk sorting"),
// bukan ORDER BY SQL. Digunakan di GET /api/karyawan/:id/komponen-gaji (F7).
//
// Diurutkan berdasarkan nilai Nominal mentah sebagaimana tersimpan di
// database (bukan hasil kalkulasi terhadap gaji_pokok) — konsisten dengan
// desain endpoint ini yang murni menampilkan daftar komponen milik satu
// karyawan, terlepas dari konteks payroll/gaji_pokok tertentu.
//
// Mengurutkan in-place (memodifikasi slice yang diberikan langsung).
// Stabil: dua komponen dengan Nominal yang sama akan mempertahankan urutan
// relatif aslinya (sifat alami insertion sort, tidak butuh penanganan
// tambahan).
//
// Kompleksitas:
//   - Waktu, worst case:   O(n^2) — data terurut menaik/acak buruk, tiap
//     elemen baru harus digeser melewati hampir semua elemen yang sudah
//     terurut sebelum menemukan posisinya.
//   - Waktu, average case: O(n^2).
//   - Waktu, best case:    O(n) — data sudah terurut menurun (kondisi yang
//     dicari fungsi ini); tiap elemen baru sudah berada di posisi yang
//     benar sehingga inner loop langsung berhenti tanpa pergeseran.
//   - Ruang:               O(1) auxiliary — hanya butuh satu variabel
//     sementara (`key`) per iterasi, sorting dilakukan in-place tanpa
//     alokasi slice/array baru.
//
// Catatan desain: insertion sort dipilih (bukan quicksort/mergesort) karena
// scope data realistis untuk endpoint ini kecil (komponen gaji per satu
// karyawan, biasanya belasan baris, bukan ribuan) — di skala ini
// kesederhanaan implementasi lebih relevan daripada kompleksitas
// asimtotik O(n log n) algoritma yang lebih rumit.
func SortKomponenGajiByNominalDesc(list []model.KomponenGaji) {
	for i := 1; i < len(list); i++ {
		key := list[i]
		j := i - 1

		// Geser elemen yang Nominal-nya lebih kecil dari key ke kanan,
		// sampai ditemukan posisi yang tepat untuk key (descending).
		for j >= 0 && list[j].Nominal.LessThan(key.Nominal) {
			list[j+1] = list[j]
			j--
		}
		list[j+1] = key
	}
}
