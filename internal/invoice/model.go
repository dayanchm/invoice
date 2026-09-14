package invoice

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// Party describes either the seller or the customer.
type Party struct {
	Name    string `yaml:"name" json:"name"`
	Address string `yaml:"address" json:"address"`
	VAT     string `yaml:"vat" json:"vat,omitempty"`
	Phone   string `yaml:"phone" json:"phone,omitempty"`
	Email   string `yaml:"email" json:"email,omitempty"`
	Website string `yaml:"website"`
}

// Item uses whole quantities and integer cents; no floating-point arithmetic.
type Item struct {
	Description    string `yaml:"description" json:"description"`
	Quantity       int64  `yaml:"quantity" json:"quantity"`
	UnitPriceCents int64  `yaml:"unit_price_cents" json:"unit_price_cents"`
}

// TotalCents is safe for items validated by New.
func (i Item) TotalCents() int64 { return i.Quantity * i.UnitPriceCents }

// Invoice is a snapshot of the data used to issue one invoice.
type Invoice struct {
	Number         string    `json:"number"`
	IssueDate      time.Time `json:"issue_date"`
	DueDate        time.Time `json:"due_date"`
	Period         string    `json:"period"`
	Seller         Party     `json:"seller"`
	Customer       Party     `json:"customer"`
	Currency       string    `json:"currency"`
	Items          []Item    `json:"items"`
	TaxBasisPoints int64     `json:"tax_basis_points"`
	SubtotalCents  int64     `json:"subtotal_cents"`
	TaxCents       int64     `json:"tax_cents"`
	TotalCents     int64     `json:"total_cents"`
	Paid           bool      `json:"paid"`
	PaymentDetails string    `json:"payment_details,omitempty"`
	Notes          string    `json:"notes,omitempty"`
}

// BalanceDueCents returns the outstanding amount after payment in full.
func (i Invoice) BalanceDueCents() int64 {
	if i.Paid {
		return 0
	}
	return i.TotalCents
}

// New validates a draft and calculates its totals. The archive assigns Number.
func New(c Config, date time.Time, period string) (Invoice, error) {
	if date.IsZero() || date.Year() < 1 || date.Year() > 9999 {
		return Invoice{}, fmt.Errorf("invoice date must have a year between 0001 and 9999")
	}
	serviceMonth, err := time.Parse("2006-01", period)
	if err != nil {
		return Invoice{}, fmt.Errorf("period must use YYYY-MM: %w", err)
	}
	if serviceMonth.Year() < 1 {
		return Invoice{}, fmt.Errorf("period must have a year between 0001 and 9999")
	}
	for _, p := range []struct {
		label string
		party Party
	}{{"seller", c.Seller}, {"customer", c.Customer}} {
		if strings.TrimSpace(p.party.Name) == "" || strings.TrimSpace(p.party.Address) == "" {
			return Invoice{}, fmt.Errorf("%s.name and %s.address are required in config", p.label, p.label)
		}
	}
	// This first version supports currencies with two decimal places.
	switch c.Currency {
	case "CHF", "EUR", "USD", "GBP", "TRY":
	default:
		return Invoice{}, fmt.Errorf("currency must be CHF, EUR, USD, GBP or TRY")
	}
	if c.PaymentTerms != "" && c.PaymentTerms != "days" && c.PaymentTerms != "end_of_month" {
		return Invoice{}, fmt.Errorf("payment_terms must be days or end_of_month")
	}
	if c.PaymentTermsDays < 0 || c.PaymentTermsDays > 365 {
		return Invoice{}, fmt.Errorf("payment_terms_days must be between 0 and 365")
	}
	if c.TaxBasisPoints < 0 || c.TaxBasisPoints > 10000 {
		return Invoice{}, fmt.Errorf("tax_basis_points must be between 0 and 10000")
	}
	if len(c.Items) == 0 {
		return Invoice{}, fmt.Errorf("at least one invoice item is required")
	}
	var subtotal int64
	for n, item := range c.Items {
		if strings.TrimSpace(item.Description) == "" || item.Quantity <= 0 || item.UnitPriceCents <= 0 {
			return Invoice{}, fmt.Errorf("item %d needs a description, positive quantity and positive unit_price_cents", n+1)
		}
		if item.UnitPriceCents > math.MaxInt64/item.Quantity || item.TotalCents() > math.MaxInt64-subtotal {
			return Invoice{}, fmt.Errorf("item %d exceeds the supported amount", n+1)
		}
		subtotal += item.TotalCents()
	}
	// Split before multiplying to avoid overflow. Round tax half up to one cent.
	tax := (subtotal/10000)*c.TaxBasisPoints + ((subtotal%10000)*c.TaxBasisPoints+5000)/10000
	if tax > math.MaxInt64-subtotal {
		return Invoice{}, fmt.Errorf("invoice total exceeds the supported amount")
	}
	date = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	dueDate := date.AddDate(0, 0, c.PaymentTermsDays)
	if c.PaymentTerms == "end_of_month" {
		// Day zero of the following month is the last day of the service month.
		dueDate = time.Date(serviceMonth.Year(), serviceMonth.Month()+1, 0, 0, 0, 0, 0, time.UTC)
	}
	if dueDate.Year() > 9999 {
		return Invoice{}, fmt.Errorf("due date exceeds year 9999")
	}
	return Invoice{
		IssueDate: date, DueDate: dueDate, Period: period,
		Seller: c.Seller, Customer: c.Customer, Currency: c.Currency,
		Items: append([]Item(nil), c.Items...), TaxBasisPoints: c.TaxBasisPoints,
		SubtotalCents: subtotal, TaxCents: tax, TotalCents: subtotal + tax,
		PaymentDetails: c.PaymentDetails, Notes: c.Notes,
	}, nil
}
