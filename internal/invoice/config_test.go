package invoice

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadConfigRejectsAmbiguousInput(t *testing.T) {
	for _, input := range []string{
		"curreny: CHF\n",
		"customer:\n  nam: typo\n",
		"currency: CHF\ncurrency: EUR\n",
		"currency: CHF\n---\ncurrency: EUR\n",
		"items:\n  - quantity: 1.5\n",
		"items:\n  - unit_price_cents: 10.5\n",
		"",
	} {
		t.Run(input, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(input), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadConfig(path); err == nil {
				t.Fatal("expected config error")
			}
		})
	}
}

func TestCustomerConfigAndEscapedTemplate(t *testing.T) {
	c, err := LoadConfig("../../data/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	want := Party{
		Name: "Example Client", Address: "Example Street 1, Genève",
		Email: "billing@example.com",
	}
	if c.Customer != want {
		t.Fatalf("customer = %+v", c.Customer)
	}
	c.Seller = Party{Name: "Example <script>alert(1)</script>", Address: "Example address"}
	c.Customer.VAT = "EXAMPLE-VAT-ID"
	c.Items[0].UnitPriceCents = 125000
	// Keep this template test independent of the user's payment schedule.
	c.PaymentTerms, c.PaymentTermsDays = "days", 30
	inv, err := New(c, time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC), "2026-01")
	if err != nil {
		t.Fatal(err)
	}
	inv.Number = "INV-2026-001"
	tmpl, err := LoadTemplate("../../templates/invoice.html")
	if err != nil {
		t.Fatal(err)
	}
	html, err := RenderHTML(tmpl, inv)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(html), "<script>") || !strings.Contains(string(html), "&lt;script&gt;") {
		t.Fatal("seller text was not HTML-escaped")
	}
	for _, text := range []string{want.Name, want.Address, c.Customer.VAT, "1250.00 CHF", inv.Number, "02 Mar 2026"} {
		if !strings.Contains(string(html), text) {
			t.Errorf("rendered HTML missing %q", text)
		}
	}
}
