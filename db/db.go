package db

import (
	"Dynamo/types"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/ElementalMP4/loggerino/log"
	_ "github.com/mattn/go-sqlite3"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

var db *sql.DB

func ensureMigrationTable() error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename TEXT PRIMARY KEY
		)
	`)
	return err
}

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

func migrationApplied(filename string) (bool, error) {
	row := db.QueryRow(
		`SELECT 1 FROM schema_migrations WHERE filename = ?`,
		filename,
	)

	var dummy int
	err := row.Scan(&dummy)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

func recordMigration(filename string) error {
	_, err := db.Exec(
		`INSERT INTO schema_migrations (filename) VALUES (?)`,
		filename,
	)
	return err
}

func runMigrations() error {
	if err := ensureMigrationTable(); err != nil {
		return err
	}

	files, err := getSortedMigrationFiles()
	if err != nil {
		return fmt.Errorf("reading migrations: %w", err)
	}

	for _, file := range files {
		applied, err := migrationApplied(file)
		if err != nil {
			return err
		}
		if applied {
			continue
		}

		content, err := migrationFS.ReadFile("migrations/" + file)
		if err != nil {
			return err
		}

		log.Infof("db-migration", "Applying migration %s", file)
		_, err = db.Exec(string(content))
		if err != nil {
			return fmt.Errorf("Failed to apply migration %s - %v", file, err)
		}
		err = recordMigration(file)
		if err != nil {
			return fmt.Errorf("Failed to record migration %s - %v", file, err)
		}
		log.Okf("db-migration", "Applied migration %s", file)

	}

	return nil
}

func InitDB() error {
	var err error
	db, err = sql.Open("sqlite3", "dynamo.db?_foreign_keys=on")
	if err != nil {
		return err
	}

	return runMigrations()
}

func DB() *sql.DB {
	return db
}

func ScanCircuitRow(row *sql.Row) (types.RoutingCircuit, error) {
	var c types.RoutingCircuit
	err := row.Scan(
		&c.ID,
		&c.Name,
		&c.Type,
		&c.Enabled,
		&c.InInterface,
		&c.Protocol,
		&c.SrcPort,
		&c.RedirectPort,
		&c.DestIP,
		&c.DestPort,
	)
	return c, err
}

func ScanCircuitRows(rows *sql.Rows) (types.RoutingCircuit, error) {
	var c types.RoutingCircuit
	err := rows.Scan(
		&c.ID,
		&c.Name,
		&c.Type,
		&c.Enabled,
		&c.InInterface,
		&c.Protocol,
		&c.SrcPort,
		&c.RedirectPort,
		&c.DestIP,
		&c.DestPort,
	)
	return c, err
}
