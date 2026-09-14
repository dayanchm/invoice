package invoice

import (
	"math"
	"testing"
	"time"
)

func validConfig() Config {
	return Config{
		Seller:   Party{Name: "Example seller", Address: "Example address"},
		Customer: Party{Name: "Example Client", Address: "Example Street 1, Genève"},
		Currency: "CHF", PaymentTermsDays: 30,
		Items: []Item{{Description: "Services", Quantity: 3, UnitPriceCents: 101}},
	}
}

func TestNewTotalsAndSnapshot(t *testing.T) {
	c := validConfig()
	c.TaxBasisPoints = 750
	c.Items = append(c.Items, Item{Description: "Support", Quantity: 1, UnitPriceCents: 100})
	date := time.Date(2026, time.December, 15, 12, 0, 0, 0, time.UTC)
	inv, err := New(c, date, "2026-11")
	if err != nil {
		t.Fatal(err)
	}
	if inv.SubtotalCents != 403 || inv.TaxCents != 30 || inv.TotalCents != 433 {
		t.Fatalf("incorrect totals: %+v", inv)
	}
	if inv.DueDate.Format(time.DateOnly) != "2027-01-14" || inv.Period != "2026-11" {
		t.Fatalf("incorrect dates: %+v", inv)
	}
	c.Items[0].Description = "Changed later"
	if inv.Items[0].Description != "Services" {
		t.Fatal("snapshot shares the config item slice")
	}
}

func TestNewEndOfMonthDueDate(t *testing.T) {
	for _, tc := range []struct{ issued, period, due string }{
		{"2026-09-14", "2026-09", "2026-09-30"},
		{"2026-01-15", "2026-01", "2026-01-31"},
		{"2026-02-01", "2026-02", "2026-02-28"},
		{"2028-02-15", "2028-02", "2028-02-29"},
		{"2026-12-15", "2026-12", "2026-12-31"},
		{"2026-09-30", "2026-09", "2026-09-30"},
		{"2026-09-14", "2025-03", "2025-03-31"},
		{"2026-09-14", "2026-08", "2026-08-31"},
		{"2026-09-14", "2024-02", "2024-02-29"},
		{"2026-09-14", "2025-02", "2025-02-28"},
		{"2026-01-15", "2025-12", "2025-12-31"},
		{"2026-09-14", "9999-12", "9999-12-31"},
	} {
		t.Run(tc.issued+"/"+tc.period, func(t *testing.T) {
			c := validConfig()
			c.PaymentTerms = "end_of_month"
			date, err := time.Parse(time.DateOnly, tc.issued)
			if err != nil {
				t.Fatal(err)
			}
			inv, err := New(c, date, tc.period)
			if err != nil {
				t.Fatal(err)
			}
			if got := inv.DueDate.Format(time.DateOnly); got != tc.due {
				t.Fatalf("due date = %s, want %s", got, tc.due)
			}
			if !inv.IssueDate.Equal(date) {
				t.Fatalf("issue date = %s, want %s", inv.IssueDate, date)
			}
		})
	}
}

func TestNewTaxRoundingAndLargeAmounts(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		amount, rate, wantTax int64
	}{
		{"below half cent", 1, 4999, 0},
		{"half cent rounds up", 1, 5000, 1},
		{"large amount without multiplication overflow", math.MaxInt64 / 2, 10000, math.MaxInt64 / 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := validConfig()
			c.Items[0].Quantity, c.Items[0].UnitPriceCents = 1, tc.amount
			c.TaxBasisPoints = tc.rate
			inv, err := New(c, time.Now(), "2026-01")
			if err != nil {
				t.Fatal(err)
			}
			if inv.TaxCents != tc.wantTax {
				t.Fatalf("tax = %d, want %d", inv.TaxCents, tc.wantTax)
			}
		})
	}
}

func TestNewRejectsInvalidData(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Config)
	}{
		{"missing seller", func(c *Config) { c.Seller.Name = " " }},
		{"missing customer", func(c *Config) { c.Customer.Address = "" }},
		{"unsupported currency", func(c *Config) { c.Currency = "JPY" }},
		{"negative terms", func(c *Config) { c.PaymentTermsDays = -1 }},
		{"unknown payment terms", func(c *Config) { c.PaymentTerms = "unknown" }},
		{"negative tax", func(c *Config) { c.TaxBasisPoints = -1 }},
		{"excessive tax", func(c *Config) { c.TaxBasisPoints = 10001 }},
		{"no items", func(c *Config) { c.Items = nil }},
		{"zero quantity", func(c *Config) { c.Items[0].Quantity = 0 }},
		{"zero price", func(c *Config) { c.Items[0].UnitPriceCents = 0 }},
		{"negative price", func(c *Config) { c.Items[0].UnitPriceCents = -1 }},
		{"line overflow", func(c *Config) { c.Items[0].UnitPriceCents = math.MaxInt64 }},
		{"subtotal overflow", func(c *Config) {
			c.Items[0].Quantity, c.Items[0].UnitPriceCents = 1, math.MaxInt64
			c.Items = append(c.Items, Item{Description: "More", Quantity: 1, UnitPriceCents: 1})
		}},
		{"total overflow", func(c *Config) {
			c.Items[0].Quantity, c.Items[0].UnitPriceCents = 1, math.MaxInt64
			c.TaxBasisPoints = 1
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := validConfig()
			tc.change(&c)
			if _, err := New(c, time.Now(), "2026-01"); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
