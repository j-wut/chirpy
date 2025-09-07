-- +goose Up

ALTER TABLE users
ADD COLUMN hashed_password text not null;

-- +goose Down
ALTER TABLE users
DROP COLUMN hashed_password;
