# Invoice

Generate monthly PDF invoices with Go. Seller, customer, and service details live
in YAML; the design lives in `templates/invoice.html` as HTML/CSS.

## Project structure

```text
invoice/
├── cmd/invoice/main.go       # CLI and invoice generation
├── templates/invoice.html   # Print-ready A4 HTML/CSS template
├── data/
│   ├── config.example.yaml  # Example configuration included in Git
│   └── config.yaml          # Your local configuration, ignored by Git
├── invoices/2026/           # Permanent archive, grouped by year
├── internal/
│   ├── invoice/             # Models, YAML, calculations, templates, archive
│   └── pdf/                 # HTML → PDF through Chrome/Chromium
├── go.mod
├── go.sum
└── README.md
```

## Setup and usage

Requires Go 1.25 or later and a local Chrome/Chromium installation. Run commands
from the project root; default paths are relative to the working directory.

```sh
go mod download

# First-time setup; -n preserves an existing local configuration.
cp -n data/config.example.yaml data/config.yaml
```

Edit `data/config.yaml`: replace the example seller and customer details, then
set `items[].unit_price_cents`. Optional bank or payment information belongs in
`payment_details`. Missing seller details and zero prices are rejected.

```sh
# Invoice for a specific month:
go run ./cmd/invoice -date 2026-01-31 -period 2026-01

# The following month; the invoice number increases automatically:
go run ./cmd/invoice -date 2026-02-28 -period 2026-02

# Use today's date and the current month:
go run ./cmd/invoice

# List all options:
go run ./cmd/invoice -help
```

The application searches PATH and common macOS application locations for the
browser. For other installations, set `-chrome` or `CHROME_BIN`:

```sh
go run ./cmd/invoice -chrome "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
```

Other options: `-config`, `-template`, and `-out`. If `-period` is omitted, it
defaults to the invoice month.

The example uses `payment_terms: end_of_month`: payment is due on the last day of
the issue month. An invoice dated September 14 is due September 30. February and
leap years are handled automatically. For a fixed number of days, use
`payment_terms: days` with `payment_terms_days: 30`. If `payment_terms` is omitted,
day-based calculation applies.

Every run creates a new invoice, including repeated runs for the same month.
Automatic monthly scheduling is outside the scope of this version.

## Local data and Git

Only `data/config.example.yaml` is intended to be versioned. Real configuration
and other files under `data/` are ignored. Generated invoices are also ignored;
only `.gitkeep` placeholders are allowed in the archive tree. Your existing local
configuration and invoices stay on disk. Keep a separate backup of the archive.

Tests use the example configuration and do not require your private config.

## Numbering and preserving previous invoices

```text
invoices/2026/
├── INV-2026-001/
│   ├── invoice.json
│   ├── invoice.html
│   └── invoice.pdf
└── INV-2026-002/
    ├── invoice.json
    ├── invoice.html
    └── invoice.pdf
```

Each invoice gets its own directory. JSON stores a snapshot of the seller,
customer, and calculated amounts. HTML stores the rendered design. Config and
template changes affect future invoices only.

- Numbering continues from the highest existing number in the **issue year**.
  Each year starts at `001`; numbers continue to `1000` after `999`.
- `os.Mkdir` reserves directories atomically, giving concurrent processes
  different numbers. Existing flat files such as `INV-2026-001.pdf` also count.
- Files are published from a temporary file in the same directory using
  `os.Link`. An existing destination causes an error; files and symlinks are
  never overwritten. Use a local filesystem that supports hard links.
- PDF is published last. Its presence marks a completed invoice. Errors or
  interruptions may leave an empty or incomplete directory. That number stays
  reserved, and the next run uses a new number, so gaps are possible.
- There is no command to update, delete, or regenerate archived invoices.
  Preserve and back up all archive directories, including incomplete ones.
  External changes depend on operating system permissions; this project does
  not provide filesystem-level immutability.

## Models and amounts

The main models are `Party`, `Item`, `Invoice`, and `Config`. Amounts use `int64`
in the smallest currency unit: `125000` means `1250.00 CHF`. Quantities must be
positive whole numbers, and prices must be positive. This version supports CHF,
EUR, USD, GBP, and TRY, all with two decimal places.

`tax_basis_points` sets one invoice-wide tax rate: `100` means `1%`. Tax is
calculated on the subtotal and rounded half up to the nearest cent. The rate is
not determined automatically; the initial value is `0`. Calculations avoid
`float64` and reject overflow. Discounts, fractional quantities, and multiple
tax rates are outside the scope of this version.

YAML field names are validated. Unknown fields, duplicate fields, multiple YAML
documents, and fractional values in integer fields are rejected.

## PDF approach

The standard library's `html/template` renders the invoice and automatically
escapes text. `internal/pdf` uses `os/exec` to call the local browser's
[headless PDF command](https://developer.chrome.com/docs/automation-and-testing/headless-cli).
The browser handles modern CSS and A4 printing. No Node.js, external PDF service,
or Go browser automation library is required.

Each render uses a separate temporary browser profile with a 60-second timeout.
Chrome writes only to a temporary directory. If it stays open after printing,
the application reads the completed PDF and stops that temporary process.
Your personal browser profile is not used. Browser updates can affect new PDFs;
archived PDFs remain unchanged.

The template includes its own CSS and uses no remote fonts, assets, or
JavaScript. Keep resources inline in custom templates to make archived HTML
self-contained.

The only Go runtime dependency is `go.yaml.in/yaml/v3 v3.0.4`. Under the
[YAML project's version policy](https://github.com/yaml/go-yaml#version-intentions),
v3 keeps a stable API and receives security fixes; new development happens in
v4. This project pins the stable v3 release.

## Verification

```sh
go test -race ./...
go vet ./...

# Optional integration test with a real browser:
INVOICE_TEST_CHROME="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" go test ./internal/pdf -run TestChromeRender -v
```

Tests cover concurrent numbering, yearly sequences, overwrite protection,
integer calculations, month-end due dates, and HTML escaping. The real browser
test is skipped unless its environment variable is set. For sample invoices,
use a separate directory such as `-out /tmp/invoice-demo`.
# invoice
