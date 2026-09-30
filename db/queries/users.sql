-- name: GetUserByID :one
SELECT * FROM users
 WHERE id = $1 AND deleted_at IS NULL;

-- name: GetUserByPhone :one
SELECT * FROM users
 WHERE phone_e164 = $1 AND deleted_at IS NULL;

-- name: ListUserRoles :many
SELECT role, scope, scope_id FROM user_roles
 WHERE user_id = $1
 ORDER BY role, scope;
