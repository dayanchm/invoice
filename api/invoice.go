package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	invoicecore "github.com/dayanchm/invoice/internal/invoice"
)

const maxRequestBytes = 1 << 20

var invoiceNumberPattern = regexp.MustCompile(`^(.*?)([0-9]+)$`)

type request struct {
	Number          string        `json:"number"`
	Period          string        `json:"period"`
	PeriodEnd       string        `json:"periodEnd"`
	IssueDate       string        `json:"issueDate"`
	Terms           string        `json:"terms"`
	DueDate         string        `json:"dueDate"`
	Paid            bool          `json:"paid"`
	SellerName      string        `json:"sellerName"`
	SellerAddress   string        `json:"sellerAddress"`
	SellerEmail     string        `json:"sellerEmail"`
	SellerPhone     string        `json:"sellerPhone"`
	SellerVAT       string        `json:"sellerVAT"`
	CustomerName    string        `json:"customerName"`
	CustomerAddress string        `json:"customerAddress"`
	CustomerEmail   string        `json:"customerEmail"`
	CustomerPhone   string        `json:"customerPhone"`
	CustomerVAT     string        `json:"customerVAT"`
	Currency        string        `json:"currency"`
	Tax             string        `json:"tax"`
	PaymentDetails  string        `json:"paymentDetails"`
	Notes           string        `json:"notes"`
	Items           []requestItem `json:"items"`
}

type requestItem struct {
	Description string `json:"description"`
	Quantity    string `json:"quantity"`
	Price       string `json:"price"`
}

type response struct {
	Invoices []responseInvoice `json:"invoices"`
}

type responseInvoice struct {
	Number          string `json:"number"`
	Period          string `json:"period"`
	IssueDate       string `json:"issueDate"`
	DueDate         string `json:"dueDate"`
	Paid            bool   `json:"paid"`
	SubtotalCents   string `json:"subtotalCents"`
	TaxCents        string `json:"taxCents"`
	TotalCents      string `json:"totalCents"`
	BalanceDueCents string `json:"balanceDueCents"`
}

// Handler validates and calculates printable invoices in a Vercel Go Function.
func Handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeError(w, http.StatusMethodNotAllowed, "Use POST to prepare invoices.")
		return
	}

	var input request
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid invoice data: "+decodeError(err))
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "The request must contain one JSON document.")
		return
	}

	result, err := prepare(input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(result)
}

func prepare(input request) (response, error) {
	issueDate, err := time.Parse(time.DateOnly, input.IssueDate)
	if err != nil || issueDate.Year() < 1900 {
		return response{}, fmt.Errorf("invoice date must be a valid date from year 1900 onward")
	}
	periods, err := periods(input.Period, input.PeriodEnd)
	if err != nil {
		return response{}, err
	}
	numbers, err := invoiceNumbers(strings.TrimSpace(input.Number), len(periods))
	if err != nil {
		return response{}, err
	}
	taxBasisPoints, err := decimalHundredths(input.Tax, "tax rate")
	if err != nil || taxBasisPoints > 10000 {
		return response{}, fmt.Errorf("tax rate must be between 0 and 100 with up to two decimals")
	}
	if len(input.Items) == 0 || len(input.Items) > 50 {
		return response{}, fmt.Errorf("provide between 1 and 50 items")
	}
	items := make([]invoicecore.Item, len(input.Items))
	for index, item := range input.Items {
		quantity, err := strconv.ParseInt(item.Quantity, 10, 64)
		if err != nil || quantity < 1 || quantity > 999999 {
			return response{}, fmt.Errorf("item %d quantity must be a whole number between 1 and 999999", index+1)
		}
		price, err := decimalHundredths(item.Price, "unit price")
		if err != nil || price <= 0 {
			return response{}, fmt.Errorf("item %d unit price must be greater than zero with up to two decimals", index+1)
		}
		items[index] = invoicecore.Item{Description: item.Description, Quantity: quantity, UnitPriceCents: price}
	}
	config := invoicecore.Config{
		Seller: invoicecore.Party{Name: input.SellerName, Address: input.SellerAddress,
			Email: input.SellerEmail, Phone: input.SellerPhone, VAT: input.SellerVAT},
		Customer: invoicecore.Party{Name: input.CustomerName, Address: input.CustomerAddress,
			Email: input.CustomerEmail, Phone: input.CustomerPhone, VAT: input.CustomerVAT},
		Currency: input.Currency, TaxBasisPoints: taxBasisPoints, Items: items,
		PaymentDetails: input.PaymentDetails, Notes: input.Notes,
	}
	switch input.Terms {
	case "end_of_month":
		config.PaymentTerms = "end_of_month"
	case "30_days":
		config.PaymentTerms, config.PaymentTermsDays = "days", 30
	case "on_receipt":
		config.PaymentTerms = "days"
	case "custom":
		config.PaymentTerms = "days"
	default:
		return response{}, fmt.Errorf("choose valid payment terms")
	}

	customDueDate := time.Time{}
	if input.Terms == "custom" {
		customDueDate, err = time.Parse(time.DateOnly, input.DueDate)
		if err != nil || customDueDate.Year() < 1900 || customDueDate.Before(issueDate) {
			return response{}, fmt.Errorf("custom due date must be on or after the invoice date")
		}
	}

	result := response{Invoices: make([]responseInvoice, len(periods))}
	for index, period := range periods {
		inv, err := invoicecore.New(config, issueDate, period)
		if err != nil {
			return response{}, err
		}
		inv.Number, inv.Paid = numbers[index], input.Paid
		if !customDueDate.IsZero() {
			inv.DueDate = customDueDate
		}
		result.Invoices[index] = responseInvoice{
			Number: inv.Number, Period: inv.Period,
			IssueDate: inv.IssueDate.Format(time.DateOnly), DueDate: inv.DueDate.Format(time.DateOnly), Paid: inv.Paid,
			SubtotalCents: strconv.FormatInt(inv.SubtotalCents, 10), TaxCents: strconv.FormatInt(inv.TaxCents, 10),
			TotalCents: strconv.FormatInt(inv.TotalCents, 10), BalanceDueCents: strconv.FormatInt(inv.BalanceDueCents(), 10),
		}
	}
	return result, nil
}

func periods(first, last string) ([]string, error) {
	start, err := time.Parse("2006-01", first)
	if err != nil || start.Year() < 1900 {
		return nil, fmt.Errorf("first service month must be valid and from year 1900 onward")
	}
	if last == "" {
		last = first
	}
	end, err := time.Parse("2006-01", last)
	if err != nil || end.Year() < 1900 {
		return nil, fmt.Errorf("last service month must be valid and from year 1900 onward")
	}
	if start.After(end) {
		return nil, fmt.Errorf("last service month cannot be earlier than the first")
	}
	var result []string
	for month := start; !month.After(end); month = month.AddDate(0, 1, 0) {
		result = append(result, month.Format("2006-01"))
		if len(result) > 120 {
			return nil, fmt.Errorf("a range can contain up to 120 months")
		}
	}
	return result, nil
}

func invoiceNumbers(first string, count int) ([]string, error) {
	match := invoiceNumberPattern.FindStringSubmatch(first)
	if match == nil {
		return nil, fmt.Errorf("invoice number must end with digits")
	}
	start, err := strconv.ParseUint(match[2], 10, 64)
	if err != nil || start > ^uint64(0)-uint64(count-1) {
		return nil, fmt.Errorf("invoice number is too large")
	}
	result := make([]string, count)
	for index := range result {
		result[index] = match[1] + fmt.Sprintf("%0*d", len(match[2]), start+uint64(index))
	}
	return result, nil
}

func decimalHundredths(value, label string) (int64, error) {
	value = strings.ReplaceAll(strings.TrimSpace(value), ",", ".")
	parts := strings.Split(value, ".")
	if len(parts) > 2 || len(parts) == 0 || parts[0] == "" || len(parts[0]) > 12 || (len(parts) == 2 && (len(parts[1]) == 0 || len(parts[1]) > 2)) {
		return 0, fmt.Errorf("invalid %s", label)
	}
	for _, part := range parts {
		if strings.Trim(part, "0123456789") != "" {
			return 0, fmt.Errorf("invalid %s", label)
		}
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s", label)
	}
	fraction := "00"
	if len(parts) == 2 {
		fraction = parts[1] + strings.Repeat("0", 2-len(parts[1]))
	}
	cents, _ := strconv.ParseInt(fraction, 10, 64)
	if whole > (1<<63-1-cents)/100 {
		return 0, fmt.Errorf("%s is too large", label)
	}
	return whole*100 + cents, nil
}

func decodeError(err error) string {
	var maxBytesError *http.MaxBytesError
	if errors.As(err, &maxBytesError) {
		return "request is too large"
	}
	return err.Error()
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
