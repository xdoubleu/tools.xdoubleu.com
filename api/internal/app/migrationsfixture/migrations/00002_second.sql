-- +goose Up
CREATE TABLE migfixture.second (id int);

-- +goose Down
DROP TABLE migfixture.second;
