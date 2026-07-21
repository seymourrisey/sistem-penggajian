package handler

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/go-playground/validator/v10"
)

// translateBindError menerjemahkan error dari c.ShouldBindJSON (validator
// required, type mismatch JSON, error parsing decimal.Decimal, JSON syntax
// error, dll) menjadi pesan bahasa Indonesia yang aman dikirim ke client —
// tidak membocorkan nama struct internal atau pesan mentah dari library.
//
// Fix bug #2, #3, #4, #5 (buglist: sebelumnya semua handler
// langsung mengirim err.Error() dari ShouldBindJSON, yang membocorkan
// pesan seperti "Key: 'xxxRequest.Field' Error:Field validation for
// 'Field' failed on the 'required' tag" atau "error decoding string
// '\"\"': can't convert \"\" to decimal".
func translateBindError(err error) string {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		return "input tidak lengkap: pastikan semua field wajib sudah diisi"
	}

	var ute *json.UnmarshalTypeError
	if errors.As(err, &ute) {
		return fmt.Sprintf("field '%s' memiliki tipe data yang salah", ute.Field)
	}

	var se *json.SyntaxError
	if errors.As(err, &se) {
		return "format JSON tidak valid"
	}

	// Fallback: mencakup kasus lain yang tidak match 3 tipe di atas, contoh
	// error dari decimal.Decimal.UnmarshalJSON saat gagal parse nilai
	// (mis. nominal/gaji_pokok dikirim sebagai string kosong ""), yang
	// balik sebagai error generik dari package decimal, bukan tipe error
	// yang bisa di-detect spesifik lewat errors.As.
	return "data yang dikirim tidak valid, periksa kembali format dan isi field"
}
