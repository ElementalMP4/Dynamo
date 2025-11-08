BEGIN;

CREATE TABLE IF NOT EXISTS local_dns (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    record_data TEXT NOT NULL,
    record_type TEXT NOT NULL CHECK (record_type IN ('A','CNAME','TXT','SRV'))
);

COMMIT;