package saves

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// zipEpoch is the fixed timestamp of every zip entry. With sorted entries
// and fixed times, the same folder always produces the same bytes, so an
// unchanged folder save never looks new to the server.
var zipEpoch = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)

// ZipDir writes a deterministic zip of dir's contents to w.
func ZipDir(dir string, w io.Writer) error { return ZipPrefix(dir, "", w) }

// hasPrefix reports whether a top-level entry belongs to a prefix save.
func hasPrefix(rel, prefix string) bool {
	if prefix == "" {
		return true
	}
	top := strings.SplitN(rel, "/", 2)[0]
	return strings.HasPrefix(top, prefix)
}

// ZipPrefix writes a deterministic zip of the entries in dir whose top-level
// name starts with prefix ("" for all), e.g. a PSP game's SAVEDATA folders.
func ZipPrefix(dir, prefix string, w io.Writer) error {
	var files []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if rel, _ := filepath.Rel(dir, p); d.IsDir() && rel != "." && !hasPrefix(filepath.ToSlash(rel), prefix) {
			return filepath.SkipDir
		}
		if d.Type().IsRegular() {
			rel, err := filepath.Rel(dir, p)
			if err != nil {
				return err
			}
			if rel = filepath.ToSlash(rel); hasPrefix(rel, prefix) {
				files = append(files, rel)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	sort.Strings(files)
	zw := zip.NewWriter(w)
	for _, rel := range files {
		hdr := &zip.FileHeader{Name: rel, Method: zip.Deflate, Modified: zipEpoch}
		hdr.SetMode(0o644)
		fw, err := zw.CreateHeader(hdr)
		if err != nil {
			return err
		}
		f, err := os.Open(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			return err
		}
		_, err = io.Copy(fw, f)
		f.Close()
		if err != nil {
			return err
		}
	}
	return zw.Close()
}

// UnzipDir replaces dir's contents with the zip at src. Entries that would
// escape dir are rejected.
func UnzipDir(src, dir string) error {
	zr, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer zr.Close()
	tmp := dir + ".rommate-tmp"
	if err := os.RemoveAll(tmp); err != nil {
		return err
	}
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return err
	}
	for _, f := range zr.File {
		name := filepath.FromSlash(f.Name)
		if filepath.IsAbs(name) || name == ".." || strings.HasPrefix(filepath.Clean(name), ".."+string(filepath.Separator)) {
			os.RemoveAll(tmp)
			return fmt.Errorf("unsafe path in save zip: %q", f.Name)
		}
		dest := filepath.Join(tmp, name)
		if strings.HasSuffix(f.Name, "/") {
			if err := os.MkdirAll(dest, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		if err := extract(f, dest); err != nil {
			os.RemoveAll(tmp)
			return err
		}
	}
	// Swap in the new folder only after everything extracted.
	old := dir + ".rommate-old"
	_ = os.RemoveAll(old)
	if err := os.Rename(dir, old); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.Rename(tmp, dir); err != nil {
		_ = os.Rename(old, dir)
		return err
	}
	return os.RemoveAll(old)
}

func extract(f *zip.File, dest string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, rc); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// UnzipPrefix replaces the top-level entries of dir that start with prefix
// with the contents of the zip at src, leaving all other entries alone.
func UnzipPrefix(src, dir, prefix string) error {
	tmp := dir + ".rommate-new"
	if err := os.RemoveAll(tmp); err != nil {
		return err
	}
	if err := UnzipDir(src, tmp); err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	// Check every new entry belongs to the prefix before touching anything.
	newEntries, err := os.ReadDir(tmp)
	if err != nil {
		return err
	}
	for _, e := range newEntries {
		if !strings.HasPrefix(e.Name(), prefix) {
			return fmt.Errorf("save zip contains %q outside %s*", e.Name(), prefix)
		}
	}
	old, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range old {
		if strings.HasPrefix(e.Name(), prefix) {
			if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
				return err
			}
		}
	}
	for _, e := range newEntries {
		if err := os.Rename(filepath.Join(tmp, e.Name()), filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}
