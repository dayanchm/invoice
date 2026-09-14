package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func validRequest() request {
	return request{
		Number: "INV-2026-001", Period: "2025-03", PeriodEnd: "2026-08",
		IssueDate: "2026-09-14", Terms: "end_of_month", Paid: true,
		SellerName: "Seller", SellerAddress: "Address",
		CustomerName: "Customer", CustomerAddress: "Address",
		Currency: "CHF", Tax: "0",
		Items: []requestItem{{Description: "Monthly services", Quantity: "1", Price: "1250.00"}},
	}
}

func TestPreparePaidRange(t *testing.T) {
	result, err := prepare(validRequest())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Invoices) != 18 {
		t.Fatalf("invoice count = %d", len(result.Invoices))
	}
	first, last := result.Invoices[0], result.Invoices[len(result.Invoices)-1]
	if first.Number != "INV-2026-001" || first.Period != "2025-03" || first.DueDate != "2025-03-31" {
		t.Fatalf("first invoice = %+v", first)
	}
	if last.Number != "INV-2026-018" || last.Period != "2026-08" || last.DueDate != "2026-08-31" {
		t.Fatalf("last invoice = %+v", last)
	}
	for _, inv := range result.Invoices {
		if inv.IssueDate != "2026-09-14" || !inv.Paid || inv.TotalCents != "125000" || inv.BalanceDueCents != "0" {
			t.Fatalf("invoice = %+v", inv)
		}
	}
}

func TestHandler(t *testing.T) {
	body, err := json.Marshal(validRequest())
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	Handler(recorder, httptest.NewRequest(http.MethodPost, "/api/invoice", bytes.NewReader(body)))
	if recorder.Code != http.StatusOK || recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status = %d, headers = %v, body = %s", recorder.Code, recorder.Header(), recorder.Body.String())
	}
	var result response
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil || len(result.Invoices) != 18 {
		t.Fatalf("response = %+v, error = %v", result, err)
	}
}

func TestPrepareRejectsInvalidInput(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*request)
	}{
		{"reversed range", func(r *request) { r.Period, r.PeriodEnd = "2026-08", "2025-03" }},
		{"bad number", func(r *request) { r.Number = "invoice" }},
		{"bad price", func(r *request) { r.Items[0].Price = "12.345" }},
		{"bad terms", func(r *request) { r.Terms = "later" }},
		{"custom due before issue", func(r *request) { r.Terms, r.DueDate = "custom", "2026-09-13" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := validRequest()
			test.change(&input)
			if _, err := prepare(input); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
