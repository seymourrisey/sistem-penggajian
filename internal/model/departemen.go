package model

import "time"

// Departemen merepresentasikan satu baris pada tabel departemen — master
// data organisasi murni tanpa histori transaksional yang melekat langsung
// padanya, sehingga menggunakan hard-delete (berbeda dengan Karyawan yang
// soft-delete)
type Departemen struct {
	ID        int       `json:"id" db:"departemen_id"`
	Nama      string    `json:"nama" db:"nama_departemen"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}
