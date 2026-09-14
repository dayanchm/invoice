package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"invoice/internal/invoice"
	"invoice/internal/pdf"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "invoice:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("invoice", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "data/config.yaml", "YAML configuration file")
	templatePath := flags.String("template", "templates/invoice.html", "HTML/CSS invoice template")
	outputDir := flags.String("out", "invoices", "invoice archive directory")
	dateValue := flags.String("date", time.Now().Format(time.DateOnly), "issue date (YYYY-MM-DD)")
	period := flags.String("period", "", "service month (YYYY-MM); defaults to the issue month")
	from := flags.String("from", "", "first service month (YYYY-MM, inclusive); requires -to")
	to := flags.String("to", "", "last service month (YYYY-MM, inclusive); requires -from")
	paid := flags.Bool("paid", false, "mark each invoice as paid in full")
	chromePath := flags.String("chrome", os.Getenv("CHROME_BIN"), "Chrome/Chromium executable")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %v", flags.Args())
	}
	date, err := time.Parse(time.DateOnly, *dateValue)
	if err != nil {
		return fmt.Errorf("invalid -date; use YYYY-MM-DD: %w", err)
	}
	periods, err := servicePeriods(date, *period, *from, *to)
	if err != nil {
		return err
	}
	config, err := invoice.LoadConfig(*configPath)
	if err != nil {
		return err
	}
	drafts := make([]invoice.Invoice, 0, len(periods))
	for _, month := range periods {
		inv, err := invoice.New(config, date, month)
		if err != nil {
			return err
		}
		inv.Paid = *paid
		drafts = append(drafts, inv)
	}
	tmpl, err := invoice.LoadTemplate(*templatePath)
	if err != nil {
		return err
	}
	chrome, err := pdf.FindChrome(*chromePath)
	if err != nil {
		return err
	}
	for _, inv := range drafts {
		if err := ctx.Err(); err != nil {
			return err
		}
		archive, err := invoice.Reserve(*outputDir, date.Year())
		if err != nil {
			return err
		}
		inv.Number = archive.Number
		if err := generate(ctx, archive, inv, tmpl, chrome); err != nil {
			return fmt.Errorf("service month %s: %s remains reserved at %s; the next run will use a new number: %w", inv.Period, inv.Number, archive.Dir, err)
		}
		fmt.Fprintln(stdout, filepath.Join(archive.Dir, "invoice.pdf"))
	}
	return nil
}

func generate(ctx context.Context, archive invoice.Archive, inv invoice.Invoice, tmpl *template.Template, chrome pdf.Chrome) error {
	html, err := invoice.RenderHTML(tmpl, inv)
	if err != nil {
		return err
	}
	/* snapshot, err := json.MarshalIndent(inv, "", "  ")
	if err != nil {
		return fmt.Errorf("encode invoice snapshot: %w", err)
	}
	if err := archive.Write("invoice.json", append(snapshot, '\n')); err != nil {
		return err
	} */
	if err := archive.Write("invoice.html", html); err != nil {
		return err
	}
	document, err := chrome.Render(ctx, html)
	if err != nil {
		return err
	}
	// Publish PDF last: its presence indicates a completed invoice.
	return archive.Write("invoice.pdf", document)
}
