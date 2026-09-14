package pdf

import (
	"bytes"
	"context"
	"os"
	"testing"
)

func TestChromeRender(t *testing.T) {
	path := os.Getenv("INVOICE_TEST_CHROME")
	if path == "" {
		t.Skip("set INVOICE_TEST_CHROME to run the real browser integration test")
	}
	chrome, err := FindChrome(path)
	if err != nil {
		t.Fatal(err)
	}
	data, err := chrome.Render(context.Background(), []byte(`<!doctype html><meta charset="utf-8"><style>@page { size: A4; }</style><h1>Invoice — Genève</h1>`))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 1000 || !bytes.HasPrefix(data, []byte("%PDF-")) || !bytes.Contains(data, []byte("%%EOF")) {
		t.Fatal("expected a complete PDF")
	}
}
