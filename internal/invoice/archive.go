package invoice

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Archive is a newly reserved invoice directory. Never remove it on failure:
// even an incomplete invoice keeps its number permanently reserved.
type Archive struct {
	Number string
	Dir    string
}

// Reserve allocates the next number for a year. Mkdir is exclusive and atomic,
// so concurrent invocations cannot reserve the same number.
func Reserve(root string, year int) (Archive, error) {
	if year < 1 || year > 9999 {
		return Archive{}, fmt.Errorf("year must be between 0001 and 9999")
	}
	dir := filepath.Join(root, fmt.Sprintf("%04d", year))
	if err := os.MkdirAll(dir, 0700); err != nil {
		return Archive{}, fmt.Errorf("create year directory: %w", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return Archive{}, fmt.Errorf("read invoice archive: %w", err)
	}
	prefix := fmt.Sprintf("INV-%04d-", year)
	var highest int64
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}
		suffix := strings.TrimPrefix(entry.Name(), prefix)
		// Also respect any pre-existing flat PDF, HTML or JSON invoice files.
		suffix = strings.TrimSuffix(suffix, filepath.Ext(suffix))
		if suffix == "" || strings.Trim(suffix, "0123456789") != "" {
			continue
		}
		n, err := strconv.ParseInt(suffix, 10, 64)
		if err != nil {
			return Archive{}, fmt.Errorf("invalid archived invoice number %q: %w", entry.Name(), err)
		}
		if n > highest {
			highest = n
		}
	}
	for highest < math.MaxInt64 {
		highest++
		number := fmt.Sprintf("%s%03d", prefix, highest)
		path := filepath.Join(dir, number)
		if err := os.Mkdir(path, 0700); errors.Is(err, os.ErrExist) {
			continue
		} else if err != nil {
			return Archive{}, fmt.Errorf("reserve invoice number: %w", err)
		}
		return Archive{Number: number, Dir: path}, nil
	}
	return Archive{}, fmt.Errorf("invoice sequence exhausted for year %d", year)
}

// Write publishes a complete file without replacing an existing path, including
// symlinks. A hard link from a temporary file in the same directory is atomic.
func (a Archive) Write(name string, data []byte) error {
	switch name {
	case "invoice.json", "invoice.html", "invoice.pdf":
	default:
		return fmt.Errorf("unsupported archive file %q", name)
	}
	f, err := os.CreateTemp(a.Dir, ".pending-*")
	if err != nil {
		return fmt.Errorf("create temporary archive file: %w", err)
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("sync %s: %w", name, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close %s: %w", name, err)
	}
	if err := os.Link(f.Name(), filepath.Join(a.Dir, name)); err != nil {
		return fmt.Errorf("publish %s (existing files are never replaced): %w", name, err)
	}
	return nil
}
