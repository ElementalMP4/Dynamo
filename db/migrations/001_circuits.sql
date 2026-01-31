BEGIN;

CREATE TABLE IF NOT EXISTS routing_circuits (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('REDIRECT', 'DNAT')),
    enabled BOOLEAN NOT NULL DEFAULT 0,

    in_interface TEXT,
    protocol TEXT NOT NULL CHECK (protocol IN ('tcp', 'udp')),
    src_port INTEGER NOT NULL,

    redirect_port INTEGER,
    dest_ip TEXT,
    dest_port INTEGER
);


COMMIT;