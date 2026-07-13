-- migrations/001_init_schema.sql
-- Sistem Informasi Penggajian — Initial Schema
-- 4 tabel inti: departemen, karyawan, komponen_gaji, payroll

CREATE TABLE departemen (
    id          SERIAL PRIMARY KEY,
    nama        VARCHAR(100) NOT NULL UNIQUE,
    created_at  TIMESTAMP DEFAULT NOW()
);

CREATE TABLE karyawan (
    id              SERIAL PRIMARY KEY,
    nip             VARCHAR(20) NOT NULL UNIQUE,
    nama            VARCHAR(100) NOT NULL,
    departemen_id   INT NOT NULL REFERENCES departemen(id),
    jabatan         VARCHAR(50) NOT NULL,
    gaji_pokok      NUMERIC(12,2) NOT NULL CHECK (gaji_pokok >= 0),
    tanggal_masuk   DATE NOT NULL,
    status          VARCHAR(20) DEFAULT 'aktif',
    created_at      TIMESTAMP DEFAULT NOW(),
    updated_at      TIMESTAMP DEFAULT NOW()
);

CREATE TABLE komponen_gaji (
    id          SERIAL PRIMARY KEY,
    karyawan_id INT NOT NULL REFERENCES karyawan(id),
    jenis       VARCHAR(20) NOT NULL CHECK (jenis IN ('tunjangan','potongan')),
    nama        VARCHAR(50) NOT NULL,
    nominal     NUMERIC(12,2) NOT NULL CHECK (nominal >= 0),
    is_persen   BOOLEAN DEFAULT FALSE
);

CREATE TABLE payroll (
    id              SERIAL PRIMARY KEY,
    karyawan_id     INT NOT NULL REFERENCES karyawan(id),
    periode         DATE NOT NULL,
    gaji_pokok      NUMERIC(12,2) NOT NULL,
    total_tunjangan NUMERIC(12,2) NOT NULL,
    total_potongan  NUMERIC(12,2) NOT NULL,
    gaji_bersih     NUMERIC(12,2) NOT NULL,
    status          VARCHAR(20) DEFAULT 'draft',
    created_at      TIMESTAMP DEFAULT NOW(),
    UNIQUE(karyawan_id, periode)
);

-- Index — persiapan skalabilitas query (WHERE/JOIN pada kolom ini)
CREATE INDEX idx_karyawan_departemen ON karyawan(departemen_id);
CREATE INDEX idx_payroll_karyawan_periode ON payroll(karyawan_id, periode);
CREATE INDEX idx_komponen_karyawan ON komponen_gaji(karyawan_id);
