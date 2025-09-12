-- name: CreateUser :one
INSERT INTO users (id, created_at, updated_at, email, hashed_password)
VALUES (
    gen_random_uuid(),
    now(),
    now(),
    $1,
    $2
)
RETURNING *;

-- name: GetUser :one
SELECT * FROM users WHERE email = $1;

-- name: ChangeEmailPassword :exec
UPDATE users set email = $2, hashed_password = $3 WHERE id = $1;

-- name: ResetUsers :exec
DELETE FROM users;
