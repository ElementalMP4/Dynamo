package db

import (
	"Dynamo/log"
	"Dynamo/types"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

var (
	db *sql.DB
)

func getSortedMigrationFiles() ([]string, error) {
	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, entry.Name())
		}
	}

	sort.Strings(files)
	return files, nil
}

func applyMigration(filename string) error {
	content, err := migrationFS.ReadFile("migrations/" + filename)
	if err != nil {
		return err
	}

	_, err = db.Exec(string(content))
	return err
}

func runMigrations() error {
	files, err := getSortedMigrationFiles()
	if err != nil {
		return fmt.Errorf("reading migrations: %w", err)
	}

	for _, file := range files {
		f := file
		msg := "Apply migration " + f
		log.LogDuration(func() (string, error) {
			return msg, applyMigration(f)
		})
	}
	return nil
}

func InitDB() error {
	var err error
	db, err = sql.Open("sqlite3", "dynamo.db")
	if err != nil {
		return err
	}

	runMigrations()
	return nil
}

// GetAllDnsRecords loads all records from the local_dns table and
// returns them as a slice of types.DnsRecord.
func GetAllDnsRecords() ([]types.DnsRecord, error) {
	rows, err := db.Query("SELECT id, name, record_data, record_type FROM local_dns")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []types.DnsRecord
	for rows.Next() {
		var id sql.NullInt64
		var name sql.NullString
		var data sql.NullString
		var rtype sql.NullString
		if err := rows.Scan(&id, &name, &data, &rtype); err != nil {
			return nil, err
		}
		var rec types.DnsRecord
		if data.Valid && data.String != "" {
			if err := json.Unmarshal([]byte(data.String), &rec); err != nil {
				log.LogError(fmt.Sprintf("failed to unmarshal record id %v: %v", id, err))
				continue
			}
		} else {
			rec = types.DnsRecord{}
		}
		if id.Valid {
			rec.ID = id.Int64
		}
		if name.Valid {
			rec.Name = name.String
		}
		if rec.Type == "" && rtype.Valid {
			rec.Type = rtype.String
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateDnsRecord inserts a new record into the database and sets rec.ID.
func CreateDnsRecord(rec *types.DnsRecord) (int64, error) {
	b, err := json.Marshal(rec)
	if err != nil {
		return 0, err
	}
	res, err := db.Exec("INSERT INTO local_dns(name, record_data, record_type) VALUES (?, ?, ?)", rec.Name, string(b), rec.Type)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	rec.ID = id
	return id, nil
}

// UpdateDnsRecord updates an existing record by id.
func UpdateDnsRecord(id int64, rec *types.DnsRecord) error {
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	_, err = db.Exec("UPDATE local_dns SET name = ?, record_data = ?, record_type = ? WHERE id = ?", rec.Name, string(b), rec.Type, id)
	return err
}

// DeleteDnsRecord deletes a record by id.
func DeleteDnsRecord(id int64) error {
	_, err := db.Exec("DELETE FROM local_dns WHERE id = ?", id)
	return err
}
