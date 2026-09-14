package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
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

func TestGenerationFailureKeepsHTMLAndNumber(t *testing.T) {
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
	data, err := os.ReadFile(filepath.Join(archive.Dir, "invoice.html"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"INV-2026-001", "1250.00 CHF"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("archived HTML missing %q", want)
		}
	}
	if _, err := os.Stat(filepath.Join(archive.Dir, "invoice.pdf")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed rendering published a PDF: %v", err)
	}
	next, err := invoice.Reserve(root, 2026)
	if err != nil || next.Number != "INV-2026-002" {
		t.Fatalf("next = %+v, %v", next, err)
	}
}

func TestRunRejectsInvalidPeriodsBeforeReservation(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"missing end", []string{"-from", "2025-03"}, "-from and -to must be provided together"},
		{"missing start", []string{"-to", "2026-08"}, "-from and -to must be provided together"},
		{"conflicting period", []string{"-period", "2026-01", "-from", "2025-03", "-to", "2026-08"}, "-period cannot be combined"},
		{"reversed range", []string{"-from", "2026-08", "-to", "2025-03"}, "-from must be before or equal to -to"},
		{"invalid start", []string{"-from", "2025-13", "-to", "2026-08"}, "invalid -from"},
		{"invalid end", []string{"-from", "2025-03", "-to", "2026-8"}, "invalid -to"},
		{"year zero", []string{"-from", "0000-01", "-to", "2026-08"}, "invalid -from"},
		{"invalid single month", []string{"-period", "2026-02-01"}, "invalid -period"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			args := append([]string{"-date", "2026-09-14", "-out", root, "-config", filepath.Join(root, "missing.yaml")}, tc.args...)
			err := run(context.Background(), args, io.Discard, io.Discard)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 0 {
				t.Fatalf("invalid arguments changed the archive: %v, %v", entries, err)
			}
		})
	}
}

func TestRunMonthlyInvoices(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake browser requires a POSIX shell")
	}
	root := t.TempDir()
	config, err := os.ReadFile("../../data/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(root, "config.yaml")
	if err := os.WriteFile(configPath, []byte(strings.Replace(string(config), "unit_price_cents: 0", "unit_price_cents: 125000", 1)), 0600); err != nil {
		t.Fatal(err)
	}
	// Exercise the CLI and archive without depending on a browser installation.
	chromePath := filepath.Join(root, "chrome")
	if err := os.WriteFile(chromePath, []byte(`#!/bin/sh
for arg in "$@"; do
  case "$arg" in
    --print-to-pdf=*) printf '%s\n' '%PDF-1.4' '%%EOF' > "${arg#--print-to-pdf=}" ;;
  esac
done
`), 0700); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		args  []string
		first string
		count int
		paid  bool
	}{
		{"paid range", []string{"-from", "2025-03", "-to", "2026-08", "-paid"}, "2025-03", 18, true},
		{"same month range", []string{"-from", "2026-02", "-to", "2026-02"}, "2026-02", 1, false},
		{"leap year range", []string{"-from", "2024-02", "-to", "2024-03"}, "2024-02", 2, false},
		{"default month", nil, "2026-09", 1, false},
		{"paid single month", []string{"-period", "2025-12", "-paid"}, "2025-12", 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := t.TempDir()
			args := append([]string{"-config", configPath, "-template", "../../templates/invoice.html", "-out", out, "-date", "2026-09-14", "-chrome", chromePath}, tc.args...)
			var stdout strings.Builder
			if err := run(context.Background(), args, &stdout, io.Discard); err != nil {
				t.Fatal(err)
			}
			paths := strings.Split(strings.TrimSpace(stdout.String()), "\n")
			if len(paths) != tc.count {
				t.Fatalf("generated %d invoices, want %d", len(paths), tc.count)
			}
			entries, err := os.ReadDir(filepath.Join(out, "2026"))
			if err != nil || len(entries) != tc.count {
				t.Fatalf("archive entries = %v, error = %v", entries, err)
			}
			first, err := time.Parse("2006-01", tc.first)
			if err != nil {
				t.Fatal(err)
			}
			for n, path := range paths {
				number := fmt.Sprintf("INV-2026-%03d", n+1)
				if want := filepath.Join(out, "2026", number, "invoice.pdf"); path != want {
					t.Fatalf("PDF path = %q, want %q", path, want)
				}
				if _, err := os.Stat(path); err != nil {
					t.Fatal(err)
				}
				html, err := os.ReadFile(filepath.Join(filepath.Dir(path), "invoice.html"))
				if err != nil {
					t.Fatal(err)
				}
				balance := "1250.00"
				if tc.paid {
					balance = "0.00"
				}
				month := first.AddDate(0, n, 0)
				dueDate := month.AddDate(0, 1, -1).Format("02 Jan 2006")
				for _, want := range []string{number, month.Format("2006-01"), `Invoice Date :</dt><dd>14 Sep 2026</dd>`, `Due Date :</dt><dd>` + dueDate + `</dd>`, `class="total"><span>Total</span><span>1250.00 CHF</span>`, `class="balance-amount">` + balance + ` CHF</p>`} {
					if !strings.Contains(string(html), want) {
						t.Errorf("%s missing %q", number, want)
					}
				}
				if strings.Contains(string(html), ">PAID</p>") != tc.paid {
					t.Errorf("%s: wrong payment status", number)
				}
			}
		})
	}
}
