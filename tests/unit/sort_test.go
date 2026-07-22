package unit

import (
	"testing"

	"github.com/shopspring/decimal"

	"github.com/seymourrisey/sistem-penggajian/internal/model"
	"github.com/seymourrisey/sistem-penggajian/internal/util"
)

func komponen(id int, nominal int64) model.KomponenGaji {
	return model.KomponenGaji{ID: id, Nominal: decimal.NewFromInt(nominal)}
}

// nominals mengekstrak urutan Nominal (sebagai int64) dari slice hasil sort,
// untuk perbandingan ringkas terhadap urutan yang diharapkan.
func nominals(list []model.KomponenGaji) []int64 {
	result := make([]int64, len(list))
	for i, k := range list {
		result[i] = k.Nominal.IntPart()
	}
	return result
}

// ids mengekstrak urutan ID — dipakai khusus test stabilitas, di mana
// Nominal-nya sengaja sama sehingga urutan ID adalah satu-satunya cara
// membuktikan elemen tidak tertukar posisi relatifnya.
func ids(list []model.KomponenGaji) []int {
	result := make([]int, len(list))
	for i, k := range list {
		result[i] = k.ID
	}
	return result
}

func assertIntSliceEqual(t *testing.T, got, want []int64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("panjang hasil = %d, want %d (got=%v)", len(got), len(want), got)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("index %d = %d, want %d (got=%v, want=%v)", i, got[i], want[i], got, want)
		}
	}
}

func TestSortKomponenGajiByNominalDesc(t *testing.T) {
	tests := []struct {
		name  string
		input []model.KomponenGaji
		want  []int64
	}{
		{
			name: "acak: harus terurut descending",
			input: []model.KomponenGaji{
				komponen(1, 500000),
				komponen(2, 100000),
				komponen(3, 750000),
				komponen(4, 250000),
			},
			want: []int64{
				750000,
				500000,
				250000,
				100000},
		},
		{
			name: "sudah terurut descending: best case, tidak boleh berubah urutan",
			input: []model.KomponenGaji{
				komponen(1, 900000),
				komponen(2, 500000),
				komponen(3, 100000),
			},
			want: []int64{
				900000,
				500000,
				100000,
			},
		},
		{
			name: "terurut ascending (worst case): harus dibalik total",
			input: []model.KomponenGaji{
				komponen(1, 100000),
				komponen(2, 500000),
				komponen(3, 900000),
			},
			want: []int64{
				900000,
				500000,
				100000,
			},
		},
		{
			name:  "slice kosong: tidak boleh panic",
			input: []model.KomponenGaji{},
			want:  []int64{},
		},
		{
			name: "satu elemen: tidak berubah",
			input: []model.KomponenGaji{
				komponen(1, 42000),
			},
			want: []int64{42000},
		},
		{
			name: "semua nominal sama: tidak boleh panic, urutan tetap valid",
			input: []model.KomponenGaji{
				komponen(1, 100000),
				komponen(2, 100000),
				komponen(3, 100000),
			},
			want: []int64{100000, 100000, 100000},
		},
		{
			name: "melibatkan nominal nol (edge case is_persen=true dengan nominal 0%)",
			input: []model.KomponenGaji{
				komponen(1, 0),
				komponen(2, 500000),
				komponen(3, 0),
			},
			want: []int64{
				500000, 0, 0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			util.SortKomponenGajiByNominalDesc(tt.input)
			assertIntSliceEqual(t, nominals(tt.input), tt.want)
		})
	}
}

// TestSortKomponenGajiByNominalDesc_Stable memverifikasi sifat stabil:
// dua+ elemen dengan Nominal yang identik harus mempertahankan urutan
// relatif aslinya (insertion sort stabil secara alami, tapi harus dibuktikan
// eksplisit, bukan diasumsikan dari nama algoritma).
func TestSortKomponenGajiByNominalDesc_Stable(t *testing.T) {
	// ID 10, 20, 30 semua Nominal 100000 (sama), diselingi elemen dengan
	// Nominal beda supaya insertion sort benar-benar melakukan pergeseran,
	// bukan cuma kebetulan tetap di posisi awal.
	input := []model.KomponenGaji{
		komponen(10, 100000),
		komponen(99, 999000), // Nominal lebih besar, harus naik ke depan
		komponen(20, 100000),
		komponen(30, 100000),
	}

	util.SortKomponenGajiByNominalDesc(input)

	wantNominals := []int64{
		999000,
		100000,
		100000,
		100000,
	}
	assertIntSliceEqual(t, nominals(input), wantNominals)

	// Di antara tiga elemen ber-Nominal sama, urutan ID harus tetap 10, 20, 30
	// (urutan asli), bukan tertukar akibat pergeseran saat elemen 99 disisipkan.
	gotIDs := ids(input)
	wantIDs := []int{10, 20, 30}
	for i, id := range gotIDs[1:] { // skip index 0 (elemen 99)
		if id != wantIDs[i] {
			t.Errorf("urutan ID di antara nominal sama tidak stabil: got=%v, want=%v", gotIDs[1:], wantIDs)
			break
		}
	}
}

// TestSortKomponenGajiByNominalDesc_InPlace memverifikasi bahwa sorting
// memodifikasi slice asli langsung (in-place), bukan mengembalikan copy baru
// tanpa menyentuh input — konsisten dengan dokumentasi fungsi dan konsumsi
// dari handler (yang akan memanggil fungsi ini lalu langsung serialize
// slice yang sama ke JSON).
func TestSortKomponenGajiByNominalDesc_InPlace(t *testing.T) {
	input := []model.KomponenGaji{komponen(1, 100000), komponen(2, 900000)}
	original := input // slice header sama, backing array sama

	util.SortKomponenGajiByNominalDesc(input)

	if original[0].Nominal.IntPart() != 900000 {
		t.Errorf("slice asli tidak ikut termodifikasi (in-place gagal): original[0].Nominal = %s", original[0].Nominal)
	}
}
