-- setup-db-user-privilege-terbatas.sql
-- Jalankan manual via pgAdmin4, connect ke database "payroll_db"
-- aplikasi terhubung pakai user dedicated, bukan superuser "postgres"

-- ============================================================
-- 1. Buat role/user baru untuk aplikasi
-- ============================================================
-- GANTI 'ganti_password_kuat_disini' dengan password kamu sendiri
-- sebelum dijalankan. Jangan commit password asli ke .env.example
-- atau ke repo manapun.
CREATE ROLE payroll_app WITH LOGIN PASSWORD 'ganti_password_kuat_disini';

-- ============================================================
-- 2. Batasi hak akses level database
-- ============================================================
-- CONNECT: wajib, supaya user ini bisa connect ke payroll_db sama sekali.
-- Tidak diberi CREATE di level database -> user ini tidak bisa bikin
-- schema/objek baru di luar apa yang sudah di-GRANT eksplisit.
GRANT CONNECT ON DATABASE payroll_db TO payroll_app;

-- ============================================================
-- 3. Hak akses level schema (default: public)
-- ============================================================
-- USAGE saja (bukan CREATE) -> user bisa "masuk" ke schema public
-- untuk akses tabel yang di-GRANT, tapi tidak bisa CREATE TABLE/DROP
-- objek baru di schema ini.
GRANT USAGE ON SCHEMA public TO payroll_app;

-- ============================================================
-- 4. Hak akses DML pada tabel aplikasi (SELECT/INSERT/UPDATE/DELETE)
-- ============================================================
-- Sengaja TIDAK termasuk DROP/CREATE/ALTER/TRUNCATE, sesuai section 2.4
-- design doc. Kalau ada tabel baru ditambahkan ke depannya, GRANT ini
-- harus diulang manual untuk tabel tsb (tidak otomatis ter-cover).
GRANT SELECT, INSERT, UPDATE, DELETE ON departemen, karyawan, komponen_gaji, payroll TO payroll_app;

-- ============================================================
-- 5. Hak akses pada sequence (WAJIB, sering terlewat)
-- ============================================================
-- Semua tabel di atas pakai kolom SERIAL (auto-increment via sequence
-- tersembunyi di belakang layar). INSERT ke tabel dengan SERIAL butuh
-- privilege USAGE (untuk nextval()) pada sequence-nya -- tanpa ini,
-- INSERT akan gagal dengan error "permission denied for sequence ...",
-- meskipun user sudah punya INSERT di tabelnya sendiri.
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO payroll_app;

-- ============================================================
-- 6. EXECUTE pada function/procedure SQL native (section 2.4)
-- ============================================================
-- Fungsi ini didemonstrasikan manual (bukan dipanggil dari Go app),
-- tapi tetap di-GRANT supaya bisa dites lewat user payroll_app juga
-- kalau diperlukan saat presentasi/demo, tanpa harus switch balik ke
-- superuser.
GRANT EXECUTE ON FUNCTION fn_hitung_gaji_bersih(NUMERIC, NUMERIC, NUMERIC) TO payroll_app;
GRANT EXECUTE ON PROCEDURE sp_generate_payroll_snapshot(INT, DATE) TO payroll_app;
GRANT SELECT ON v_laporan_gaji_departemen TO payroll_app;

-- ============================================================
-- Verifikasi cepat (jalankan manual setelah user dibuat)
-- ============================================================
-- Cek privilege yang benar-benar ter-attach ke user ini:
-- SELECT grantee, table_name, privilege_type
-- FROM information_schema.role_table_grants
-- WHERE grantee = 'payroll_app';

-- Cek user TIDAK bisa CREATE TABLE (harus gagal dengan permission denied):
-- (jalankan sebagai payroll_app, bukan sebagai superuser)
-- CREATE TABLE test_seharusnya_gagal (id INT);
