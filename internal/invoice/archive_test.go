package invoice

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestArchiveSequenceAndNoOverwrite(t *testing.T) {
	root := t.TempDir()
	first, err := Reserve(root, 2026)
	if err != nil {
		t.Fatal(err)
	}
	if first.Number != "INV-2026-001" {
		t.Fatal(first.Number)
	}
	for _, name := range []string{"invoice.json", "invoice.html", "invoice.pdf"} {
		if err := first.Write(name, []byte("original")); err != nil {
			t.Fatal(err)
		}
		if err := first.Write(name, []byte("changed")); !errors.Is(err, os.ErrExist) {
			t.Fatalf("overwrite error = %v", err)
		}
		data, err := os.ReadFile(filepath.Join(first.Dir, name))
		if err != nil || string(data) != "original" {
			t.Fatalf("archived file changed: %q, %v", data, err)
		}
	}
	second, err := Reserve(root, 2026)
	if err != nil || second.Number != "INV-2026-002" {
		t.Fatalf("second = %+v, %v", second, err)
	}
	// An interrupted run leaves an empty directory; its number is not reused.
	third, err := Reserve(root, 2026)
	if err != nil || third.Number != "INV-2026-003" {
		t.Fatalf("third = %+v, %v", third, err)
	}
	nextYear, err := Reserve(root, 2027)
	if err != nil || nextYear.Number != "INV-2027-001" {
		t.Fatalf("next year = %+v, %v", nextYear, err)
	}
}

func TestReserveUsesHighestNumberAndPasses999(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "2026", "INV-2026-998"), 0700); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(root, "2026", "INV-2026-999.pdf")
	if err := os.WriteFile(legacy, []byte("old PDF"), 0600); err != nil {
		t.Fatal(err)
	}
	archive, err := Reserve(root, 2026)
	if err != nil || archive.Number != "INV-2026-1000" {
		t.Fatalf("archive = %+v, %v", archive, err)
	}
	data, err := os.ReadFile(legacy)
	if err != nil || string(data) != "old PDF" {
		t.Fatalf("old PDF changed: %q, %v", data, err)
	}
}

func TestReserveConcurrent(t *testing.T) {
	root := t.TempDir()
	const count = 24
	results := make(chan Archive, count)
	errs := make(chan error, count)
	var wg sync.WaitGroup
	for n := 0; n < count; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			archive, err := Reserve(root, 2026)
			if err != nil {
				errs <- err
				return
			}
			results <- archive
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	seen := make(map[string]bool)
	for archive := range results {
		if seen[archive.Number] {
			t.Errorf("duplicate number %s", archive.Number)
		}
		seen[archive.Number] = true
	}
	for n := 1; n <= count; n++ {
		if !seen[fmt.Sprintf("INV-2026-%03d", n)] {
			t.Errorf("missing invoice %d", n)
		}
	}
}

func TestArchiveDoesNotFollowExistingSymlink(t *testing.T) {
	root := t.TempDir()
	archive, err := Reserve(root, 2026)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "original.pdf")
	if err := os.WriteFile(target, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(archive.Dir, "invoice.pdf")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := archive.Write("invoice.pdf", []byte("changed")); !errors.Is(err, os.ErrExist) {
		t.Fatalf("overwrite error = %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "original" {
		t.Fatalf("symlink target changed: %q, %v", data, err)
	}
}
