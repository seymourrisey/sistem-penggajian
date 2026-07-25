# API Documentation — Sistem Informasi Penggajian

**Base URL:** `http://localhost:8080/api`
**Format:** JSON (`Content-Type: application/json`)
**Autentikasi:** Tidak ada (di luar scope studi kasus ini)

---

## Konvensi Umum

### Format Tanggal
Semua field tanggal (`tanggal_masuk`, `periode`) menggunakan format **`YYYY-MM-DD`** (date-only, tanpa jam/timezone).

### Format Angka Uang
Field bertipe uang (`gaji_pokok`, `nominal`, `total_tunjangan`, `total_potongan`, `gaji_bersih`) di-serialize sebagai **string berkuotasi** (bukan number JSON mentah), karena menggunakan `decimal.Decimal` untuk presisi.
```json
{"gaji_pokok": "5000000"}
```
Bisa dikirim sebagai number literal (`5000000`) maupun string (`"5000000"`) saat request — keduanya diterima. Response selalu string.

### Kontrak Error
Semua endpoint yang gagal mengembalikan body dengan bentuk konsisten:
```json
{"error": "pesan error dalam bahasa Indonesia"}
```

**Penting soal pesan error 400** — ada 2 sumber berbeda dengan karakter pesan beda:
- **Field wajib kosong / tipe data salah saat binding JSON**: pesan **generik**, sama untuk semua field, contoh: `{"error": "input tidak lengkap: pastikan semua field wajib sudah diisi"}`. Tidak menyebutkan nama field spesifik yang bermasalah — jangan bangun logic frontend yang bergantung pada teks pesan untuk highlight field tertentu.
- **Validasi business rule di service layer** (misal `gaji_pokok` negatif, `jenis` komponen gaji tidak valid): pesan **spesifik**, contoh: `{"error": "gaji_pokok tidak boleh negatif"}`, `{"error": "jenis komponen gaji harus 'tunjangan' atau 'potongan'"}`.
- **Format JSON tidak valid**: `{"error": "format JSON tidak valid"}`
- **Tipe data field salah** (mis. kirim string ke field angka): `{"error": "field 'X' memiliki tipe data yang salah"}`

**Pesan error "id tidak valid" tidak konsisten antar endpoint** (temuan aktual dari kode, bukan seharusnya begini): endpoint `departemen` memakai `"invalid id"`, endpoint lain (`karyawan`, `komponen-gaji`, `payroll`) memakai `"id harus berupa angka"`. Frontend jangan bergantung pada teks pesan ini untuk logic apapun — cukup treat semua HTTP 400 dari path param sebagai "format ID salah".

### Mapping Status Code
| Status | Kondisi |
|---|---|
| 200 OK | Request GET/PUT/DELETE berhasil |
| 201 Created | Request POST berhasil membuat resource baru |
| 400 Bad Request | Validasi input gagal, format salah, atau referensi FK tidak valid |
| 404 Not Found | Resource yang diminta (by ID di path) tidak ditemukan |
| 409 Conflict | Duplikasi yang melanggar UNIQUE constraint atau FK constraint pada DELETE |
| 500 Internal Server Error | Kegagalan tak terduga |

---

## 1. Departemen

### 1.1 `POST /api/departemen` — Tambah departemen

**Request Body**
```json
{
  "nama": "IT"
}
```

**Response 201**
```json
{
  "id": 1,
  "nama": "IT",
  "created_at": "2026-07-25T10:00:00Z"
}
```

**Error**
| Status | Kondisi | Contoh Response |
|---|---|---|
| 400 | `nama` kosong (string benar-benar kosong `""`) | Pesan generik binding: `{"error": "input tidak lengkap: pastikan semua field wajib sudah diisi"}` |
| 400 | `nama` hanya spasi (lolos binding `required`, ditolak di service) | `{"error": "nama departemen tidak boleh kosong"}` |
| 409 | `nama` sudah terdaftar | `{"error": "nama departemen sudah terdaftar"}` |

---

### 1.2 `GET /api/departemen` — List semua departemen

**Response 200**
```json
[
  { "id": 1, "nama": "IT", "created_at": "2026-07-25T10:00:00Z" },
  { "id": 2, "nama": "HR", "created_at": "2026-07-25T10:05:00Z" }
]
```

---

### 1.3 `GET /api/departemen/:id` — Detail departemen

**Response 200**
```json
{ "id": 1, "nama": "IT", "created_at": "2026-07-25T10:00:00Z" }
```

**Error**
| Status | Kondisi |
|---|---|
| 404 | `id` tidak ditemukan |

---

### 1.4 `PUT /api/departemen/:id` — Update nama departemen

**Request Body**
```json
{ "nama": "IT & Digital" }
```

**Response 200**
```json
{ "message": "departemen berhasil di update" }
```

**Error**
| Status | Kondisi |
|---|---|
| 400 | `nama` kosong |
| 404 | `id` tidak ditemukan |
| 409 | `nama` baru bentrok dengan departemen lain |

---

### 1.5 `DELETE /api/departemen/:id` — Hapus departemen (hard-delete)

> Departemen adalah master data tanpa histori transaksional, sehingga dihapus permanen. Ditolak jika masih direferensikan karyawan aktif.

**Response 200**
```json
{ "message": "departemen berhasil di delete" }
```

**Error**
| Status | Kondisi | Contoh Response |
|---|---|---|
| 404 | `id` tidak ditemukan | `{"error": "departemen tidak ditemukan"}` |
| 409 | Masih direferensikan karyawan | `{"error": "departemen tidak dapat dihapus karena masih direferensikan karyawan"}` |

---

## 2. Karyawan

### 2.1 `POST /api/karyawan` — Tambah karyawan

**Request Body**
```json
{
  "nip": "EMP-001",
  "nama": "Budi Santoso",
  "departemen_id": 1,
  "jabatan": "Staff",
  "gaji_pokok": "5000000",
  "tanggal_masuk": "2024-01-15"
}
```

**Response 201**
```json
{
  "id": 1,
  "nip": "EMP-001",
  "nama": "Budi Santoso",
  "departemen_id": 1,
  "jabatan": "Staff",
  "gaji_pokok": "5000000",
  "tanggal_masuk": "2024-01-15",
  "status": "aktif",
  "created_at": "2026-07-25T10:00:00Z",
  "updated_at": "2026-07-25T10:00:00Z"
}
```

**Error**
| Status | Kondisi | Contoh Response |
|---|---|---|
| 400 | Field wajib benar-benar kosong (`nip`, `nama`, `jabatan`, `gaji_pokok`, `tanggal_masuk`, dll) | Pesan generik: `{"error": "input tidak lengkap: pastikan semua field wajib sudah diisi"}` |
| 400 | `nip`/`nama` hanya spasi (lolos binding, ditolak di service) | `{"error": "nip tidak boleh kosong"}` / `{"error": "nama karyawan tidak boleh kosong"}` |
| 400 | `gaji_pokok` negatif | `{"error": "gaji_pokok tidak boleh negatif"}` |
| 400 | `tanggal_masuk` format bukan `YYYY-MM-DD` | `{"error": "tanggal_masuk harus format YYYY-MM-DD"}` |
| 400 | `departemen_id` tidak valid/tidak ada | `{"error": "departemen_id tidak valid atau tidak ditemukan"}` |
| 409 | `nip` sudah terdaftar | `{"error": "nip sudah terdaftar"}` |

> `status` selalu default `"aktif"` saat create, tidak bisa di-set manual lewat body.

---

### 2.2 `GET /api/karyawan` — List karyawan (filter opsional)

**Query Param**
| Param | Tipe | Keterangan |
|---|---|---|
| `departemen` | int, opsional | Filter berdasarkan `departemen_id` |

**Contoh:** `GET /api/karyawan?departemen=1`

**Response 200**
```json
[
  {
    "id": 1, "nip": "EMP-001", "nama": "Budi Santoso",
    "departemen_id": 1, "jabatan": "Staff", "gaji_pokok": "5000000",
    "tanggal_masuk": "2024-01-15", "status": "aktif",
    "created_at": "2026-07-25T10:00:00Z", "updated_at": "2026-07-25T10:00:00Z"
  }
]
```

---

### 2.3 `GET /api/karyawan/:id` — Detail karyawan

**Response 200:** sama seperti struktur pada 2.1

**Error**
| Status | Kondisi |
|---|---|
| 400 | `id` di path bukan angka |
| 404 | `id` tidak ditemukan |

---

### 2.4 `PUT /api/karyawan/:id` — Update data karyawan

> **PENTING:** field `status` TIDAK bisa diubah lewat endpoint ini meski dikirim di body — akan diabaikan. Perubahan status hanya lewat `DELETE` (soft-delete). Ini business rule sengaja, bukan bug.

**Request Body**
```json
{
  "nip": "EMP-001",
  "nama": "Budi Santoso",
  "departemen_id": 2,
  "jabatan": "Staff Senior",
  "gaji_pokok": "6000000",
  "tanggal_masuk": "2024-01-15"
}
```

**Response 200:** struktur sama seperti 2.1, `status` tetap nilai sebelumnya.

**Error**
| Status | Kondisi |
|---|---|
| 400 | Field wajib kosong / `gaji_pokok` negatif / `departemen_id` tidak valid |
| 404 | `id` tidak ditemukan |
| 409 | `nip` baru bentrok dengan karyawan lain |

---

### 2.5 `DELETE /api/karyawan/:id` — Soft-delete (nonaktifkan karyawan)

> Karyawan tidak pernah dihapus fisik (punya histori payroll). Endpoint ini mengubah `status` jadi `"nonaktif"`.

**Response 200**
```json
{ "message": "karyawan berhasil dinonaktifkan" }
```

**Error**
| Status | Kondisi |
|---|---|
| 404 | `id` tidak ditemukan |

---

## 3. Komponen Gaji

> Komponen gaji (tunjangan/potongan) melekat ke karyawan sebagai "template" — tidak diinput ulang tiap bulan, otomatis dipakai tiap kali `payroll/generate` dijalankan untuk karyawan tersebut.

### 3.1 `POST /api/karyawan/:id/komponen-gaji` — Tambah komponen gaji

**Request Body**
```json
{
  "jenis": "tunjangan",
  "nama": "Tunjangan Transport",
  "nominal": "500000",
  "is_persen": false
}
```
> Contoh potongan persentase (mis. BPJS 2% dari gaji pokok):
```json
{
  "jenis": "potongan",
  "nama": "BPJS Kesehatan",
  "nominal": "2",
  "is_persen": true
}
```

**Response 201**
```json
{
  "id": 1,
  "karyawan_id": 1,
  "jenis": "tunjangan",
  "nama": "Tunjangan Transport",
  "nominal": "500000",
  "is_persen": false
}
```

**Error**
| Status | Kondisi | Contoh Response |
|---|---|---|
| 400 | `jenis` bukan `tunjangan`/`potongan` | `{"error": "jenis komponen gaji harus 'tunjangan' atau 'potongan'"}` |
| 400 | `nama`/`nominal` benar-benar kosong | Pesan generik: `{"error": "input tidak lengkap: pastikan semua field wajib sudah diisi"}` |
| 400 | `nama` hanya spasi (lolos binding, ditolak di service) | `{"error": "nama komponen gaji tidak boleh kosong"}` |
| 400 | `karyawan_id` (dari path `:id`) tidak valid/tidak ditemukan | `{"error": "karyawan_id tidak valid atau tidak ditemukan"}` |
| 409 | Kombinasi `karyawan_id + jenis + nama` sudah ada | `{"error": "komponen gaji dengan jenis dan nama ini sudah ada untuk karyawan tersebut"}` |

---

### 3.2 `GET /api/karyawan/:id/komponen-gaji` — List komponen gaji milik karyawan

> Response **terurut berdasarkan nominal terbesar** (algoritma sorting manual, bukan `ORDER BY` SQL).

**Response 200**
```json
[
  { "id": 2, "karyawan_id": 1, "jenis": "tunjangan", "nama": "Tunjangan Transport", "nominal": "500000", "is_persen": false },
  { "id": 1, "karyawan_id": 1, "jenis": "potongan", "nama": "BPJS Kesehatan", "nominal": "2", "is_persen": true }
]
```
Array kosong `[]` jika karyawan belum punya komponen gaji.

**Error**
| Status | Kondisi |
|---|---|
| 404 | `karyawan_id` tidak ditemukan |

---

### 3.3 `GET /api/komponen-gaji/:id` — Detail satu komponen gaji

**Response 200**
```json
{ "id": 1, "karyawan_id": 1, "jenis": "potongan", "nama": "BPJS Kesehatan", "nominal": "2", "is_persen": true }
```

**Error**
| Status | Kondisi |
|---|---|
| 404 | `id` tidak ditemukan |

---

### 3.4 `PUT /api/karyawan/:id/komponen-gaji/:komponen_id` — Koreksi komponen gaji

> Untuk kasus HR salah input nominal/persentase/jenis. Tidak ada endpoint DELETE — koreksi lewat Update, bukan hapus, supaya histori payroll yang sudah pakai komponen ini tetap tertelusuri.

**Request Body**
```json
{
  "jenis": "potongan",
  "nama": "BPJS Kesehatan",
  "nominal": "1",
  "is_persen": true
}
```

**Response 200**
```json
{ "id": 1, "karyawan_id": 1, "jenis": "potongan", "nama": "BPJS Kesehatan", "nominal": "1", "is_persen": true }
```

**Error**
| Status | Kondisi |
|---|---|
| 400 | Validasi gagal (sama seperti Create) |
| 404 | `komponen_id` tidak ditemukan, atau tidak milik `karyawan_id` di path |

---

## 4. Payroll

### 4.1 `POST /api/payroll/generate` — Generate slip gaji

> `karyawan_id` dan `periode` dikirim di body (bukan URL param). Proses: ambil gaji_pokok karyawan → ambil semua komponen gaji → hitung tunjangan/potongan (persen dihitung flat dari gaji_pokok) → simpan snapshot. Dibungkus transaksi eksplisit (COMMIT/ROLLBACK).

**Request Body**
```json
{
  "karyawan_id": "1",
  "periode": "2026-08-01"
}
```

**Response 201**
```json
{
  "id": 1,
  "karyawan_id": 1,
  "periode": "2026-08-01",
  "gaji_pokok": "5000000",
  "total_tunjangan": "500000",
  "total_potongan": "100000",
  "gaji_bersih": "5400000",
  "status": "draft",
  "created_at": "2026-07-25T10:00:00Z"
}
```

**Error**
| Status | Kondisi | Contoh Response |
|---|---|---|
| 400 | `karyawan_id`/`periode` kosong atau format salah | — |
| 404 | `karyawan_id` tidak ditemukan | `{"error": "karyawan tidak ditemukan"}` |
| 409 | Payroll untuk `karyawan_id` + `periode` ini sudah pernah digenerate | `{"error": "payroll sudah ada untuk karyawan dan periode ini"}` |

---

### 4.2 `GET /api/payroll/:karyawan_id/riwayat` — Riwayat gaji karyawan

> Query dengan JOIN ke tabel `karyawan` (menyertakan nama & nip di response).

**Response 200**
```json
[
  {
    "id": 1, "karyawan_id": 1, "nip": "EMP-001", "nama_karyawan": "Budi Santoso",
    "periode": "2026-08-01", "gaji_pokok": "5000000",
    "total_tunjangan": "500000", "total_potongan": "100000",
    "gaji_bersih": "5400000", "status": "draft", "created_at": "2026-07-25T10:00:00Z"
  }
]
```
Array kosong `[]` jika karyawan ada tapi belum pernah generate payroll.

**Error**
| Status | Kondisi | Catatan |
|---|---|---|
| 404 | `karyawan_id` tidak ditemukan | Beda dari kasus "belum ada riwayat" (200+`[]`) — lihat catatan di atas |

---

### 4.3 `GET /api/payroll/laporan` — Laporan agregat gaji per departemen

**Query Param**
| Param | Tipe | Wajib |
|---|---|---|
| `periode` | `YYYY-MM-DD` | Ya |

**Contoh:** `GET /api/payroll/laporan?periode=2026-08-01`

**Response 200**
```json
[
  {
    "departemen_id": 1,
    "nama_departemen": "IT",
    "periode": "2026-08-01",
    "jumlah_karyawan": 5,
    "total_gaji_bersih": "27000000",
    "rata_rata_gaji_bersih": "5400000"
  },
  {
    "departemen_id": 2,
    "nama_departemen": "HR",
    "periode": "2026-08-01",
    "jumlah_karyawan": 3,
    "total_gaji_bersih": "15300000",
    "rata_rata_gaji_bersih": "5100000"
  }
]
```
Query internal menggunakan JOIN 3 tabel (`payroll` → `karyawan` → `departemen`) + `GROUP BY` + `SUM`/`AVG`.

**Error**
| Status | Kondisi |
|---|---|
| 400 | `periode` kosong atau format bukan `YYYY-MM-DD` |

---

## Ringkasan Seluruh Endpoint

| Method | Path | Fungsi |
|---|---|---|
| POST | `/api/departemen` | Tambah departemen |
| GET | `/api/departemen` | List departemen |
| GET | `/api/departemen/:id` | Detail departemen |
| PUT | `/api/departemen/:id` | Update departemen |
| DELETE | `/api/departemen/:id` | Hapus departemen (hard-delete, FK-protected) |
| POST | `/api/karyawan` | Tambah karyawan |
| GET | `/api/karyawan` | List karyawan (filter `?departemen=`) |
| GET | `/api/karyawan/:id` | Detail karyawan |
| PUT | `/api/karyawan/:id` | Update karyawan (tidak bisa ubah `status`) |
| DELETE | `/api/karyawan/:id` | Soft-delete (nonaktifkan) karyawan |
| POST | `/api/karyawan/:id/komponen-gaji` | Tambah komponen gaji |
| GET | `/api/karyawan/:id/komponen-gaji` | List komponen gaji (terurut nominal) |
| GET | `/api/komponen-gaji/:id` | Detail komponen gaji |
| PUT | `/api/karyawan/:id/komponen-gaji/:komponen_id` | Koreksi komponen gaji |
| POST | `/api/payroll/generate` | Generate slip gaji |
| GET | `/api/payroll/:karyawan_id/riwayat` | Riwayat gaji karyawan |
| GET | `/api/payroll/laporan?periode=` | Laporan agregat per departemen |

---

## Catatan untuk Pengembangan Frontend

- Semua field uang perlu di-parse dari string ke number di sisi client sebelum ditampilkan/dihitung ulang (`parseFloat` cukup untuk display; hindari operasi aritmatika presisi tinggi di JS tanpa library decimal kalau perlu akurasi penuh).
- Tidak ada endpoint auth — semua endpoint bisa diakses langsung tanpa token.
- Tidak ada endpoint "generate payroll untuk semua karyawan sekaligus" — frontend perlu loop manual per `karyawan_id` kalau butuh proses banyak karyawan dalam satu periode.
- Field `status` pada `payroll` (`draft`/`final`) tersimpan di database tapi **tidak ada endpoint untuk mengubahnya** — murni artefak desain untuk kebutuhan masa depan, selalu bernilai `"draft"` saat ini.
