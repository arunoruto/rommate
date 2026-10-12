package state

import (
	"database/sql"
	"errors"
	"time"
)

// GetValue returns a stored value, or "" if unset.
func (s *Store) GetValue(key string) (string, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM kv WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return v, err
}

// SetValue stores a value.
func (s *Store) SetValue(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO kv (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// SaveKey identifies one synced save.
type SaveKey struct {
	RomID    int    `json:"rom_id"`
	Slot     string `json:"slot"`
	Emulator string `json:"emulator"`
}

// Baseline is the hash of a save when it was last in sync with the server.
type Baseline struct {
	SaveKey
	Hash     string
	SyncedAt time.Time
}

// GetBaseline returns the baseline of a save, if any.
func (s *Store) GetBaseline(k SaveKey) (Baseline, bool, error) {
	var b Baseline
	var ts int64
	err := s.db.QueryRow(`SELECT hash, synced_at FROM save_baselines WHERE rom_id = ? AND slot = ? AND emulator = ?`,
		k.RomID, k.Slot, k.Emulator).Scan(&b.Hash, &ts)
	if errors.Is(err, sql.ErrNoRows) {
		return Baseline{}, false, nil
	}
	b.SaveKey, b.SyncedAt = k, time.Unix(ts, 0)
	return b, err == nil, err
}

// PutBaseline records that a save is in sync with the server.
func (s *Store) PutBaseline(b Baseline) error {
	_, err := s.db.Exec(`INSERT INTO save_baselines (rom_id, slot, emulator, hash, synced_at) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(rom_id, slot, emulator) DO UPDATE SET hash = excluded.hash, synced_at = excluded.synced_at`,
		b.RomID, b.Slot, b.Emulator, b.Hash, b.SyncedAt.Unix())
	return err
}

// Conflict is a save that changed both locally and on the server.
type Conflict struct {
	SaveKey
	ServerSaveID    int       `json:"server_save_id"`
	ServerUpdatedAt time.Time `json:"server_updated_at"`
	LocalHash       string    `json:"local_hash"`
	DetectedAt      time.Time `json:"detected_at"`
}

// PutConflict records (or refreshes) an unresolved conflict.
func (s *Store) PutConflict(c Conflict) error {
	_, err := s.db.Exec(`INSERT INTO save_conflicts (rom_id, slot, emulator, server_save_id, server_updated_at, local_hash, detected_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(rom_id, slot, emulator) DO UPDATE SET server_save_id = excluded.server_save_id,
			server_updated_at = excluded.server_updated_at, local_hash = excluded.local_hash, detected_at = excluded.detected_at`,
		c.RomID, c.Slot, c.Emulator, c.ServerSaveID, c.ServerUpdatedAt.Unix(), c.LocalHash, c.DetectedAt.Unix())
	return err
}

// DeleteConflict removes a resolved conflict.
func (s *Store) DeleteConflict(k SaveKey) error {
	_, err := s.db.Exec(`DELETE FROM save_conflicts WHERE rom_id = ? AND slot = ? AND emulator = ?`, k.RomID, k.Slot, k.Emulator)
	return err
}

// Conflicts lists unresolved conflicts, oldest first.
func (s *Store) Conflicts() ([]Conflict, error) {
	rows, err := s.db.Query(`SELECT rom_id, slot, emulator, server_save_id, server_updated_at, local_hash, detected_at
		FROM save_conflicts ORDER BY detected_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Conflict
	for rows.Next() {
		var c Conflict
		var su, da int64
		if err := rows.Scan(&c.RomID, &c.Slot, &c.Emulator, &c.ServerSaveID, &su, &c.LocalHash, &da); err != nil {
			return nil, err
		}
		c.ServerUpdatedAt, c.DetectedAt = time.Unix(su, 0), time.Unix(da, 0)
		out = append(out, c)
	}
	return out, rows.Err()
}
