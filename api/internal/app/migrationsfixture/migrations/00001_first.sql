-- +goose Up
CREATE TABLE migfixture.first (id int);

-- +goose Down
DROP TABLE migfixture.first;
