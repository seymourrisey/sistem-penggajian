-- migrations/002_sql_native_features.sql
-- function, trigger, view, stored procedure
-- Melengkapi, bukan menggantikan, logic Go di payroll_service.go

-- ============================================================
-- FUNCTION: hitung gaji bersih (versi SQL, demonstrasi paralel dari logic Go)
-- ============================================================
CREATE OR REPLACE FUNCTION fn_hitung_gaji_bersih(
    p_gaji_pokok NUMERIC, p_tunjangan NUMERIC, p_potongan NUMERIC
) RETURNS NUMERIC AS $$
BEGIN
    RETURN p_gaji_pokok + p_tunjangan - p_potongan;
END;
$$ LANGUAGE plpgsql;

-- ============================================================
-- TRIGGER: auto-update updated_at pada karyawan saat UPDATE
-- ============================================================
CREATE OR REPLACE FUNCTION trg_set_updated_at() RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_karyawan_updated_at ON karyawan;

CREATE TRIGGER trg_karyawan_updated_at
BEFORE UPDATE ON karyawan
FOR EACH ROW EXECUTE FUNCTION trg_set_updated_at();

-- ============================================================
-- VIEW: laporan gaji per departemen (menyederhanakan query JOIN+GROUP BY yang sudah ada)
-- ============================================================
CREATE OR REPLACE VIEW v_laporan_gaji_departemen AS
SELECT d.nama_departemen AS departemen, p.periode,
       COUNT(p.karyawan_id) AS jumlah_karyawan,
       SUM(p.gaji_bersih) AS total_gaji_bersih,
       AVG(p.gaji_bersih) AS rata_rata_gaji
FROM payroll p
JOIN karyawan k ON p.karyawan_id = k.karyawan_id
JOIN departemen d ON k.departemen_id = d.departemen_id
GROUP BY d.nama_departemen, p.periode;

-- ============================================================
-- STORED PROCEDURE: contoh generate payroll snapshot via SQL murni (demonstrasi tambahan)
-- Catatan: ini demonstrasi terpisah untuk KUK, endpoint aktual /api/payroll/generate
-- tetap pakai logic Go di payroll_service.go (source-of-truth kalkulasi, ada unit test-nya)
-- ============================================================
CREATE OR REPLACE PROCEDURE sp_generate_payroll_snapshot(
    p_karyawan_id INT, p_periode DATE
)
LANGUAGE plpgsql AS $$
DECLARE
    v_gaji_pokok NUMERIC;
BEGIN
    SELECT gaji_pokok INTO v_gaji_pokok FROM karyawan WHERE karyawan_id = p_karyawan_id;

    IF v_gaji_pokok IS NULL THEN
        RAISE EXCEPTION 'karyawan_id % tidak ditemukan', p_karyawan_id;
    END IF;

    INSERT INTO payroll (karyawan_id, periode, gaji_pokok, total_tunjangan, total_potongan, gaji_bersih)
    VALUES (p_karyawan_id, p_periode, v_gaji_pokok, 0, 0, v_gaji_pokok);
END;
$$;
