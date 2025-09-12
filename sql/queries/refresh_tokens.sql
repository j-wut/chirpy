-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (token, created_at, updated_at, expires_at, user_id)
VALUES (
    $1,
    now(),
    now(),
    $2,
    $3
)
RETURNING *;

-- name: GetRefreshToken :one
SELECT * FROM refresh_tokens WHERE token = $1 AND revoked_at IS NULL;

-- name: GetRefreshTokenFromUserID :one
SELECT * FROM refresh_tokens WHERE user_id = $1 AND revoked_at IS NULL;

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens SET revoked_at = now() WHERE token = $1;

-- name: ResetRefreshTokens :exec
DELETE FROM refresh_tokens;
