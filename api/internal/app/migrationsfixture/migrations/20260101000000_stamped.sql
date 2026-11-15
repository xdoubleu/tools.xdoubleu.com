-- +goose Up
CREATE TABLE migfixture.stamped (id int);
INSERT INTO migfixture.second (id) VALUES (1);

-- +goose Down
DROP TABLE migfixture.stamped;
