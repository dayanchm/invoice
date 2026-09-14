package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"invoice/internal/invoice"
	"invoice/internal/pdf"
)

func TestRunRejectsIncompleteConfigBeforeReservation(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "incomplete.yaml")
	if err := os.WriteFile(configPath, []byte("seller: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	err := run(context.Background(), []string{
		"-config", configPath, "-out", root, "-date", "2026-01-31",
	}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "seller.name") {
		t.Fatalf("error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "2026")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid config reserved a number: %v", err)
	}
}

func TestGenerationFailureKeepsSnapshotAndNumber(t *testing.T) {
	c, err := invoice.LoadConfig("../../data/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	c.Seller = invoice.Party{Name: "Demo seller", Address: "Demo address"}
	c.Items[0].UnitPriceCents = 125000
	inv, err := invoice.New(c, time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC), "2026-01")
	if err != nil {
		t.Fatal(err)
	}
	tmpl, err := invoice.LoadTemplate("../../templates/invoice.html")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	archive, err := invoice.Reserve(root, 2026)
	if err != nil {
		t.Fatal(err)
	}
	inv.Number = archive.Number
	chrome := pdf.Chrome{Executable: filepath.Join(root, "missing-browser")}
	if err := generate(context.Background(), archive, inv, tmpl, chrome); err == nil {
		t.Fatal("expected browser error")
	}
	data, err := os.ReadFile(filepath.Join(archive.Dir, "invoice.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved invoice.Invoice
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Number != "INV-2026-001" || saved.TotalCents != 125000 {
		t.Fatalf("snapshot = %+v", saved)
	}
	if _, err := os.Stat(filepath.Join(archive.Dir, "invoice.html")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(archive.Dir, "invoice.pdf")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed rendering published a PDF: %v", err)
	}
	next, err := invoice.Reserve(root, 2026)
	if err != nil || next.Number != "INV-2026-002" {
		t.Fatalf("next = %+v, %v", next, err)
	}
}
