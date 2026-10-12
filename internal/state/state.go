// Package state records which ROMs rommate has downloaded and where.
package state

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Entry is one downloaded ROM.
type Entry struct {
	RomID        int    `json:"rom_id"`
	PlatformSlug string `json:"platform_slug"`
	Name         string `json:"name"`
	// Path is the ROM file, or its folder for multi-file ROMs.
	Path         string    `json:"path"`
	SizeBytes    int64     `json:"size_bytes"`
	DownloadedAt time.Time `json:"downloaded_at"`
}

// Store is the SQLite-backed state database.
type Store struct {
	db *sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS roms (
	rom_id        INTEGER PRIMARY KEY,
	platform_slug TEXT NOT NULL,
	name          TEXT NOT NULL,
	path          TEXT NOT NULL,
	size_bytes    INTEGER NOT NULL,
	downloaded_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS roms_platform ON roms(platform_slug);
`

// Open opens (and creates if needed) the database in dir.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	// WAL + busy timeout let the TUI and a CLI/Decky call run side by side.
	dsn := "file:" + filepath.Join(dir, "state.db") +
		"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialising state db: %w", err)
	}
	return &Store{db: db}, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// Put records a completed download.
func (s *Store) Put(e Entry) error {
	_, err := s.db.Exec(`
		INSERT INTO roms (rom_id, platform_slug, name, path, size_bytes, downloaded_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(rom_id) DO UPDATE SET
			platform_slug = excluded.platform_slug,
			name          = excluded.name,
			path          = excluded.path,
			size_bytes    = excluded.size_bytes,
			downloaded_at = excluded.downloaded_at`,
		e.RomID, e.PlatformSlug, e.Name, e.Path, e.SizeBytes, e.DownloadedAt.Unix())
	return err
}

// Get returns the entry for a ROM, or ok=false if it isn't recorded.
func (s *Store) Get(romID int) (Entry, bool, error) {
	row := s.db.QueryRow(`SELECT rom_id, platform_slug, name, path, size_bytes, downloaded_at
		FROM roms WHERE rom_id = ?`, romID)
	e, err := scan(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Entry{}, false, nil
	}
	return e, err == nil, err
}

// Delete forgets a ROM.
func (s *Store) Delete(romID int) error {
	_, err := s.db.Exec(`DELETE FROM roms WHERE rom_id = ?`, romID)
	return err
}

// All returns every recorded ROM, ordered by platform and name.
func (s *Store) All() ([]Entry, error) {
	rows, err := s.db.Query(`SELECT rom_id, platform_slug, name, path, size_bytes, downloaded_at
		FROM roms ORDER BY platform_slug, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		e, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// CountByPlatform returns how many ROMs are recorded per platform slug.
func (s *Store) CountByPlatform() (map[string]int, error) {
	rows, err := s.db.Query(`SELECT platform_slug, COUNT(*) FROM roms GROUP BY platform_slug`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var slug string
		var n int
		if err := rows.Scan(&slug, &n); err != nil {
			return nil, err
		}
		out[slug] = n
	}
	return out, rows.Err()
}

type scanner interface{ Scan(...any) error }

func scan(r scanner) (Entry, error) {
	var e Entry
	var ts int64
	err := r.Scan(&e.RomID, &e.PlatformSlug, &e.Name, &e.Path, &e.SizeBytes, &ts)
	e.DownloadedAt = time.Unix(ts, 0)
	return e, err
}
