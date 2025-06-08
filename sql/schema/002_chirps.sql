-- +goose Up
CREATE TABLE chirps (
    id uuid primary key,
    created_at timestamp not null,
    updated_at timestamp not null,
    body text not null,
    user_id uuid not null REFERENCES users ON DELETE CASCADE 
);

-- +goose Down
DROP TABLE chirps;
