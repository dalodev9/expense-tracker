CREATE TABLE IF NOT EXISTS expenses (
    id          TEXT PRIMARY KEY,
    amount      INTEGER NOT NULL,
    description TEXT NOT NULL,
    category    TEXT NOT NULL,
    date        TEXT NOT NULL
);
