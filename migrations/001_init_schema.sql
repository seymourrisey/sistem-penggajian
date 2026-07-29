-- migrations/001_init_schema.sql
-- Sistem Informasi Penggajian — Initial Schema

CREATE TABLE departemen (
    departemen_id      SERIAL PRIMARY KEY,
    nama_departemen    VARCHAR(100) NOT NULL UNIQUE,
    created_at         TIMESTAMP DEFAULT NOW()
);

CREATE TABLE karyawan (
    karyawan_id        SERIAL PRIMARY KEY,
    nip                VARCHAR(20) NOT NULL UNIQUE,
    nama_karyawan      VARCHAR(100) NOT NULL,
    departemen_id      INT NOT NULL REFERENCES departemen(departemen_id),
    jabatan            VARCHAR(50) NOT NULL,
    gaji_pokok         NUMERIC(12,2) NOT NULL CHECK (gaji_pokok >= 0),
    tanggal_masuk      DATE NOT NULL,
    status             VARCHAR(20) DEFAULT 'aktif',
    created_at         TIMESTAMP DEFAULT NOW(),
    updated_at         TIMESTAMP DEFAULT NOW()
);

CREATE TABLE komponen_gaji (
    komponen_gaji_id   SERIAL PRIMARY KEY,
    karyawan_id        INT NOT NULL REFERENCES karyawan(karyawan_id),
    jenis              VARCHAR(20) NOT NULL CHECK (jenis IN ('tunjangan','potongan')),
    nama_komponen_gaji VARCHAR(50) NOT NULL,
    nominal            NUMERIC(12,2) NOT NULL CHECK (nominal >= 0),
    is_persen          BOOLEAN DEFAULT FALSE,

    CONSTRAINT uq_komponen_gaji_karyawan_jenis_nama
        UNIQUE (karyawan_id, jenis, nama_komponen_gaji)
);

CREATE TABLE payroll (
    payroll_id         SERIAL PRIMARY KEY,
    karyawan_id        INT NOT NULL REFERENCES karyawan(karyawan_id),
    periode            DATE NOT NULL,
    gaji_pokok         NUMERIC(12,2) NOT NULL,
    total_tunjangan    NUMERIC(12,2) NOT NULL,
    total_potongan     NUMERIC(12,2) NOT NULL,
    gaji_bersih        NUMERIC(12,2) NOT NULL,
    status             VARCHAR(20) DEFAULT 'draft',
    created_at         TIMESTAMP DEFAULT NOW(),

    CONSTRAINT uq_payroll_karyawan_periode
        UNIQUE(karyawan_id, periode)
);

-- Index
CREATE INDEX idx_karyawan_departemen
ON karyawan(departemen_id);

CREATE INDEX idx_payroll_karyawan_periode
ON payroll(karyawan_id, periode);

CREATE INDEX idx_komponen_karyawan
ON komponen_gaji(karyawan_id);
