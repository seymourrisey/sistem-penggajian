package model

import "time"

type Departemen struct {
	ID        int       `json:"id" db:"id"`
	Nama      string    `json:"nama" db:"nama"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}
