package core

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/arunoruto/rommate/internal/romm"
)

// progressInterval throttles progress events so JSON consumers aren't flooded.
const progressInterval = 250 * time.Millisecond

// downloadFile fetches one file into dest via dest.part, resuming a previous
// partial download when the server supports Range requests.
func downloadFile(ctx context.Context, c *romm.Client, romID int, f romm.RomFile, dest string, progress func(int64)) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	part := dest + ".part"
	out, err := os.OpenFile(part, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()

	// The .part file doubles as a lock so two processes never write it at once.
	if err := lockFile(out); err != nil {
		return ErrBusy
	}

	offset, err := out.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}
	if offset > f.FileSizeBytes {
		offset = 0
	}

	body, _, resumed, err := c.OpenFile(ctx, romID, f, offset)
	if err != nil {
		return err
	}
	defer body.Close()
	if !resumed {
		// Server sent the whole file; start over.
		offset = 0
		if err := out.Truncate(0); err != nil {
			return err
		}
		if _, err := out.Seek(0, io.SeekStart); err != nil {
			return err
		}
	}

	pw := &progressWriter{w: out, n: offset, report: progress}
	progress(offset)
	if _, err := io.Copy(pw, body); err != nil {
		return err
	}
	progress(pw.n)
	if pw.n != f.FileSizeBytes {
		return fmt.Errorf("%s: got %d bytes, expected %d", f.FileName, pw.n, f.FileSizeBytes)
	}
	if err := out.Sync(); err != nil {
		return err
	}
	// Close (and unlock) before renaming: Windows cannot rename open files.
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(part, dest)
}

type progressWriter struct {
	w      io.Writer
	n      int64
	last   time.Time
	report func(int64)
}

func (p *progressWriter) Write(b []byte) (int, error) {
	n, err := p.w.Write(b)
	p.n += int64(n)
	if now := time.Now(); now.Sub(p.last) >= progressInterval {
		p.last = now
		p.report(p.n)
	}
	return n, err
}
