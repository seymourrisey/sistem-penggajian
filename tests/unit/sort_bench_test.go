package unit

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/seymourrisey/sistem-penggajian/internal/model"
	"github.com/seymourrisey/sistem-penggajian/internal/util"
)

// generateKomponen membuat slice model.KomponenGaji sepanjang n, dengan urutan
// nominal sesuai `order`:
//   - "random"     -> acak (representasi average case)
//   - "ascending"  -> menaik (worst case untuk sort descending: setiap elemen
//     baru harus digeser melewati SEMUA elemen yang sudah tersisip)
//   - "descending" -> menurun / sudah terurut sesuai target (best case: inner
//     loop insertion sort langsung berhenti tanpa pergeseran)
//
// Nominal sengaja dibuat unik per elemen (bukan nilai acak yang bisa
// duplikat) supaya tidak ada elemen dengan nominal sama yang bisa
// mengaburkan pengukuran worst/best case.
func generateKomponen(n int, order string) []model.KomponenGaji {
	result := make([]model.KomponenGaji, n)

	switch order {
	case "ascending":
		for i := 0; i < n; i++ {
			result[i] = komponen(i, int64(i+1)*1000)
		}
	case "descending":
		for i := 0; i < n; i++ {
			result[i] = komponen(i, int64(n-i)*1000)
		}
	case "random":
		values := make([]int64, n)
		for i := range values {
			values[i] = int64(i+1) * 1000
		}
		r := rand.New(rand.NewSource(42)) // seed tetap -> hasil deterministik antar-run
		r.Shuffle(n, func(i, j int) { values[i], values[j] = values[j], values[i] })
		for i := 0; i < n; i++ {
			result[i] = komponen(i, values[i])
		}
	default:
		panic("order tidak dikenal: " + order)
	}

	return result
}

// benchmarkSortAtSize mengukur waktu eksekusi SortKomponenGajiByNominalDesc
// pada ukuran input n tetap, untuk satu skenario order tertentu.
//
// Generate ulang copy segar di setiap iterasi (di luar timer) karena fungsi
// yang diuji bersifat in-place — kalau dipakai ulang slice yang sama tanpa
// re-copy, iterasi ke-2 dst akan selalu mengukur best case (data sudah
// terurut dari iterasi sebelumnya), bukan skenario order yang dimaksud.
func benchmarkSortAtSize(b *testing.B, n int, order string) {
	base := generateKomponen(n, order)
	data := make([]model.KomponenGaji, n)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		copy(data, base)
		b.StartTimer()

		util.SortKomponenGajiByNominalDesc(data)
	}
}

// Ukuran input diuji berjenjang (kelipatan ~2x) untuk memudahkan
// perbandingan rasio pertumbuhan waktu eksekusi terhadap N — cara empiris
// membedakan O(n) vs O(n^2): pada O(n), waktu per elemen (ns/op dibagi n)
// harusnya relatif konstan antar ukuran; pada O(n^2), ns/op akan naik
// mendekati 4x setiap kali N didobelkan.
var benchmarkSizes = []int{10, 100, 500, 1000, 2000, 4000}

// BenchmarkSort_Random — average case, representasi realistis kondisi data
// di produksi (komponen gaji diinput HR tanpa urutan tertentu).
func BenchmarkSort_Random(b *testing.B) {
	for _, n := range benchmarkSizes {
		b.Run(fmt.Sprintf("N=%d", n), func(b *testing.B) {
			benchmarkSortAtSize(b, n, "random")
		})
	}
}

// BenchmarkSort_WorstCase_Ascending — worst case teoretis O(n^2): input
// menaik, kebalikan total dari target output (descending), sehingga setiap
// elemen baru digeser melewati seluruh elemen yang sudah tersisip.
func BenchmarkSort_WorstCase_Ascending(b *testing.B) {
	for _, n := range benchmarkSizes {
		b.Run(fmt.Sprintf("N=%d", n), func(b *testing.B) {
			benchmarkSortAtSize(b, n, "ascending")
		})
	}
}

// BenchmarkSort_BestCase_Descending — best case teoretis O(n): input sudah
// terurut descending (sama seperti target), inner loop insertion sort
// langsung berhenti di iterasi pertama untuk tiap elemen.
func BenchmarkSort_BestCase_Descending(b *testing.B) {
	for _, n := range benchmarkSizes {
		b.Run(fmt.Sprintf("N=%d", n), func(b *testing.B) {
			benchmarkSortAtSize(b, n, "descending")
		})
	}
}
