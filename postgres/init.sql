-- 1. Tạo database nếu chưa tồn tại
SELECT 'CREATE DATABASE real_estate'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'real_estate')\gexec

-- 2. Chuyển ngữ cảnh kết nối sang database real_estate
\c real_estate;

-- 3. Bật extension hữu ích cho hệ thống bất động sản (tìm kiếm tiếng Việt, tạo UUID)
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "unaccent";