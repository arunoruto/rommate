// Package saves syncs emulator save files with RomM.
package saves

import (
	"archive/zip"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// Hash computes RomM's content hash of a save file, so identical saves
// negotiate as no-ops: the MD5 of the file, or for zip files the MD5 of
// "name:md5(entry)" lines over all entries sorted by name (directories
// skipped), joined with "\n". This mirrors the server's assets handler.
func Hash(path string) (string, error) {
	if zr, err := zip.OpenReader(path); err == nil {
		defer zr.Close()
		return hashZip(&zr.Reader)
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func hashZip(zr *zip.Reader) (string, error) {
	sums := map[string]string{}
	var names []string
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, "/") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		h := md5.New()
		_, err = io.Copy(h, rc)
		rc.Close()
		if err != nil {
			return "", err
		}
		sums[f.Name] = hex.EncodeToString(h.Sum(nil))
		names = append(names, f.Name)
	}
	sort.Strings(names)
	lines := make([]string, len(names))
	for i, n := range names {
		lines[i] = fmt.Sprintf("%s:%s", n, sums[n])
	}
	sum := md5.Sum([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:]), nil
}
