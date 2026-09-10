CREATE UNIQUE INDEX idx_users_single_super_admin_root
ON users(role)
WHERE role = 'SUPER_ADMIN_ROOT';
