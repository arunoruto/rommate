package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// SetValue changes one top-level setting in config.toml and leaves the rest
// of the file (comments, order, tables) as it is. value must be a bool or a
// string. A missing setting is added before the first table.
func SetValue(key string, value any) error {
	var lit string
	switch v := value.(type) {
	case bool:
		lit = strconv.FormatBool(v)
	case string:
		lit = strconv.Quote(v)
	default:
		return fmt.Errorf("unsupported value type %T", value)
	}
	path, err := Path()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	out := setLine(string(data), key, lit)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(out), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

var tableHeader = regexp.MustCompile(`^\s*\[`)

// setLine sets key = lit in the top-level part of a TOML document.
func setLine(doc, key, lit string) string {
	line := key + " = " + lit
	keyLine := regexp.MustCompile(`^\s*` + regexp.QuoteMeta(key) + `\s*=`)
	lines := strings.Split(doc, "\n")
	firstTable := len(lines)
	for i, l := range lines {
		if tableHeader.MatchString(l) {
			firstTable = i
			break
		}
		if keyLine.MatchString(l) {
			lines[i] = line
			return strings.Join(lines, "\n")
		}
	}
	// Insert after the last top-level line that isn't blank, so the setting
	// stays out of the blank lines that separate tables.
	at := firstTable
	for at > 0 && strings.TrimSpace(lines[at-1]) == "" {
		at--
	}
	lines = append(lines[:at], append([]string{line}, lines[at:]...)...)
	out := strings.Join(lines, "\n")
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return out
}
