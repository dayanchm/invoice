// Package pdf converts rendered HTML into PDF using a local Chrome/Chromium.
package pdf

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

type Chrome struct {
	Executable string
}

// FindChrome accepts an explicit executable, then searches common locations.
func FindChrome(explicit string) (Chrome, error) {
	if explicit != "" {
		path, err := exec.LookPath(explicit)
		if err != nil {
			return Chrome{}, fmt.Errorf("find Chrome %q: %w", explicit, err)
		}
		return Chrome{Executable: path}, nil
	}
	candidates := []string{"chromium", "chromium-browser", "google-chrome", "google-chrome-stable", "chrome"}
	if runtime.GOOS == "darwin" {
		candidates = append(candidates,
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium")
	}
	for _, candidate := range candidates {
		if path, err := exec.LookPath(candidate); err == nil {
			return Chrome{Executable: path}, nil
		}
	}
	return Chrome{}, fmt.Errorf("Chrome/Chromium not found; install it or set -chrome /path/to/browser (or CHROME_BIN)")
}

// Render only writes into a temporary directory; the caller owns archiving.
func (c Chrome) Render(ctx context.Context, html []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	dir, err := os.MkdirTemp("", "invoice-pdf-*")
	if err != nil {
		return nil, fmt.Errorf("create PDF workspace: %w", err)
	}
	defer os.RemoveAll(dir)
	dir, err = filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve PDF workspace: %w", err)
	}
	htmlPath := filepath.Join(dir, "invoice.html")
	if err := os.WriteFile(htmlPath, html, 0600); err != nil {
		return nil, fmt.Errorf("write temporary HTML: %w", err)
	}
	pdfPath := filepath.Join(dir, "invoice.pdf")
	urlPath := filepath.ToSlash(htmlPath)
	if runtime.GOOS == "windows" {
		urlPath = "/" + urlPath
	}
	pageURL := (&url.URL{Scheme: "file", Path: urlPath}).String()
	cmd := exec.CommandContext(ctx, c.Executable,
		"--headless", "--no-pdf-header-footer", "--no-first-run", "--no-default-browser-check",
		"--disable-extensions", "--disable-background-networking", "--disable-background-mode",
		"--use-mock-keychain", "--password-store=basic",
		"--user-data-dir="+filepath.Join(dir, "profile"),
		"--print-to-pdf="+pdfPath, pageURL,
	)
	cmd.WaitDelay = 5 * time.Second
	type result struct {
		output []byte
		err    error
	}
	done := make(chan result, 1)
	go func() {
		output, err := cmd.CombinedOutput()
		done <- result{output, err}
	}()
	// Some Chrome builds keep running after printing. Once the complete PDF is
	// available, stop our isolated browser and wait for it before cleaning up.
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case result := <-done:
			if ctx.Err() != nil {
				return nil, fmt.Errorf("PDF rendering interrupted: %w", ctx.Err())
			}
			if result.err != nil {
				return nil, fmt.Errorf("Chrome PDF rendering failed: %w\n%s", result.err, bytes.TrimSpace(result.output))
			}
			return readPDF(pdfPath)
		case <-ticker.C:
			if data, err := readPDF(pdfPath); err == nil {
				cancel()
				<-done
				return data, nil
			}
		}
	}
}

func readPDF(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read generated PDF: %w", err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF-")) || !bytes.HasSuffix(bytes.TrimSpace(data), []byte("%%EOF")) {
		return nil, fmt.Errorf("Chrome did not produce a complete PDF document")
	}
	return data, nil
}
