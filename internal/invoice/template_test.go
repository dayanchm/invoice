package invoice

import (
	"strings"
	"testing"
	"time"
)

func TestTemplatePaymentStatus(t *testing.T) {
	tmpl, err := LoadTemplate("../../templates/invoice.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		paid    bool
		balance string
	}{
		{"unpaid", false, "3.03"},
		{"paid", true, "0.00"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inv, err := New(validConfig(), time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC), "2025-03")
			if err != nil {
				t.Fatal(err)
			}
			inv.Paid = tc.paid
			html, err := RenderHTML(tmpl, inv)
			if err != nil {
				t.Fatal(err)
			}
			output := string(html)
			for _, want := range []string{
				`class="balance-amount">` + tc.balance + ` CHF</p>`,
				`class="balance-due"><span>Balance Due</span><span>` + tc.balance + ` CHF</span>`,
				`class="total"><span>Total</span><span>3.03 CHF</span>`,
			} {
				if !strings.Contains(output, want) {
					t.Errorf("rendered HTML missing %q", want)
				}
			}
			for _, marker := range []string{`>PAID</p>`, `<span>Amount Paid</span><span>3.03 CHF</span>`} {
				if strings.Contains(output, marker) != tc.paid {
					t.Errorf("payment marker %q does not match paid=%t", marker, tc.paid)
				}
			}
		})
	}
}
