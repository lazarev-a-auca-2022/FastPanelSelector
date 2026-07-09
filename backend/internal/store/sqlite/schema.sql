CREATE TABLE IF NOT EXISTS plans (
    id        TEXT PRIMARY KEY,
    location  TEXT NOT NULL,
    city      TEXT NOT NULL,
    package   TEXT NOT NULL,
    arch      TEXT NOT NULL,
    cpu_type  TEXT NOT NULL,
    cores     INTEGER NOT NULL,
    ram       INTEGER NOT NULL,
    disk      INTEGER NOT NULL,
    enabled   INTEGER NOT NULL,
    price     REAL
);
