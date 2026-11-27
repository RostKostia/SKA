// Package parse turns an owner's chat message into an intent, without any AI. It is the
// fallback when Claude isn't configured or doesn't answer, and it mirrors the mockup's
// parseInvoiceCommand rules (mockup/logic.js).
package parse

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/RostKostia/SKA/backend/internal/money"
)

type Intent string

const (
	CreateInvoice  Intent = "create_invoice"
	ProvideDetails Intent = "provide_details" // e.g. an email address Kerdos asked for
	Remind         Intent = "remind"
	Forecast       Intent = "forecast"
	Financing      Intent = "financing"
	Debtors        Intent = "debtors"
	Unknown        Intent = "unknown"
)

// Line is one invoice line. Quantity is in thousandths so it stays exact (1.5 hours = 1500).
type Line struct {
	Description    string `json:"description"`
	QuantityMilli  int64  `json:"quantity_milli"`
	Unit           string `json:"unit"` // hour, day, item or service
	UnitPriceCents int64  `json:"unit_price_cents"`
}

// NetCents is round(quantity × unit price), half up, the same as the database's generated column.
func (l Line) NetCents() int64 { return (l.QuantityMilli*l.UnitPriceCents*2 + 1000) / 2000 }

// Quantity gives the quantity as a decimal string for SQL numeric: "3", "1.5".
func (l Line) Quantity() string {
	s := strconv.FormatInt(l.QuantityMilli/1000, 10)
	if frac := l.QuantityMilli % 1000; frac != 0 {
		s += "." + strings.TrimRight(fmt.Sprintf("%03d", frac), "0")
	}
	return s
}

// Draft is an invoice the owner asked for: who, and what for.
type Draft struct {
	Client string `json:"client"`
	Lines  []Line `json:"lines"`
}

func (d Draft) TotalCents() int64 {
	var t int64
	for _, l := range d.Lines {
		t += l.NetCents()
	}
	return t
}

type Result struct {
	Intent  Intent `json:"intent"`
	Draft   *Draft `json:"draft,omitempty"`   // create_invoice
	Missing string `json:"missing,omitempty"` // create_invoice: "client" or "amount"
	Client  string `json:"client,omitempty"`  // remind: who
	All     bool   `json:"all,omitempty"`     // remind: everyone
	Email   string `json:"email,omitempty"`   // provide_details
	Source  string `json:"source"`            // "rules" or "claude"
}

var (
	verbRe   = regexp.MustCompile(`(?i)^\s*(?:invoice|bill|charge)\b[\s,:]*(.*)$`)
	wordRe   = regexp.MustCompile(`^[\p{L}][\p{L}'’\-]*`)
	timeRe   = regexp.MustCompile(`(?i)(\d+(?:[.,]\d+)?)\s*(hours?|hrs?|h|days?)\s*(?:at|@|x|×)\s*€?\s*(\d[\d.,]*)`)
	amountRe = regexp.MustCompile(`€?\s*(\d[\d.,]*)\s*(?:€|eur|euros?)?`)
	forRe    = regexp.MustCompile(`(?i)\bfor\s+(.+)$`)
	emailRe  = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	stop     = map[string]bool{"for": true, "at": true, "to": true, "x": true, "and": true}
)

// InvoiceCommand reads "Invoice Kostas €120", "Invoice Kostas, 3 hours at €40" or
// "Bill Elena €450 for menu design". ok is false when text isn't an invoice command at all.
// missing is "client" or "amount" when that part is absent.
func InvoiceCommand(text string) (d Draft, missing string, ok bool) {
	m := verbRe.FindStringSubmatch(text)
	if m == nil {
		return Draft{}, "", false
	}
	client, tail := leadingName(m[1])
	d.Client = client
	if client == "" {
		return d, "client", true
	}

	desc := ""
	if f := forRe.FindStringSubmatch(tail); f != nil {
		desc = strings.TrimSpace(strings.Trim(amountRe.ReplaceAllString(timeRe.ReplaceAllString(f[1], ""), ""), " .,!"))
		tail = tail[:len(tail)-len(f[0])] + " " + f[1]
	}

	if h := timeRe.FindStringSubmatch(tail); h != nil {
		qty, okQ := quantityMilli(h[1])
		rate, okR := money.ParseCents(h[3])
		if !okQ || !okR {
			return d, "amount", true
		}
		unit := "hour"
		if strings.HasPrefix(strings.ToLower(h[2]), "d") {
			unit = "day"
		}
		line := Line{QuantityMilli: qty, Unit: unit, UnitPriceCents: rate}
		line.Description = desc
		if desc == "" {
			plural := unit + "s"
			if qty == 1000 {
				plural = unit
			}
			line.Description = fmt.Sprintf("%s %s at %s", line.Quantity(), plural, money.Format(rate))
		}
		d.Lines = []Line{line}
		return d, "", true
	}

	a := amountRe.FindStringSubmatch(tail)
	if a == nil {
		return d, "amount", true
	}
	cents, okA := money.ParseCents(a[1])
	if !okA {
		return d, "amount", true
	}
	if desc == "" {
		desc = "Services"
	}
	d.Lines = []Line{{Description: capitalize(desc), QuantityMilli: 1000, Unit: "service", UnitPriceCents: cents}}
	return d, "", true
}

// leadingName takes the words at the start of s as a name ("Maria Savva"), stopping at a
// comma, a number, or a word like "for" or "at". It returns the name and what follows.
func leadingName(s string) (name, rest string) {
	s = strings.TrimLeft(s, " ")
	pos := 0
	var words []string
	for {
		loc := wordRe.FindStringIndex(s[pos:])
		if loc == nil {
			break
		}
		w := s[pos : pos+loc[1]]
		if stop[strings.ToLower(w)] {
			break
		}
		words = append(words, w)
		pos += loc[1]
		j := pos
		for j < len(s) && s[j] == ' ' {
			j++
		}
		if j == pos || j >= len(s) || !wordRe.MatchString(s[j:]) {
			break
		}
		next := wordRe.FindString(s[j:])
		if stop[strings.ToLower(next)] {
			break
		}
		pos = j
	}
	return strings.Join(words, " "), s[pos:]
}

func quantityMilli(s string) (int64, bool) {
	c, ok := money.ParseCents(strings.Replace(s, ",", ".", 1))
	if !ok {
		return 0, false
	}
	return c * 10, true // cents are hundredths; thousandths = ×10
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	return strings.ToUpper(string(r[0])) + string(r[1:])
}

// Detect reads any owner message.
func Detect(text string) Result {
	t := strings.TrimSpace(text)
	low := strings.ToLower(t)
	res := Result{Intent: Unknown, Source: "rules"}
	switch {
	case t == "":
		return res
	case emailRe.MatchString(t) && len(strings.Fields(t)) <= 4 && !verbRe.MatchString(t):
		res.Intent, res.Email = ProvideDetails, emailRe.FindString(t)
	case verbRe.MatchString(t):
		d, missing, _ := InvoiceCommand(t)
		res.Intent, res.Draft, res.Missing = CreateInvoice, &d, missing
	case strings.HasPrefix(low, "remind"):
		res.Intent = Remind
		rest := strings.TrimSpace(t[len("remind"):])
		switch strings.ToLower(strings.Trim(rest, " .!")) {
		case "", "all", "everyone", "everybody", "all clients", "them all", "all of them":
			res.All = true
		default:
			res.Client, _ = leadingName(rest)
			if res.Client == "" {
				res.All = true
			}
		}
	case containsAny(low, "get money", "money now", "advance", "financ", "borrow", "loan", "get €", "cash now"):
		res.Intent = Financing
	case containsAny(low, "who owes", "owes me", "owe me", "debtor", "unpaid", "outstanding"):
		res.Intent = Debtors
	case containsAny(low, "forecast", "cash", "balance", "negative", "afford", "runway", "how am i doing", "how are we doing"):
		res.Intent = Forecast
	}
	return res
}

func containsAny(s string, subs ...string) bool {
	for _, x := range subs {
		if strings.Contains(s, x) {
			return true
		}
	}
	return false
}
