package parse

import "testing"

// The first cases are the mockup's tests (mockup/tests/logic.test.js), kept identical.
func TestInvoiceCommand(t *testing.T) {
	cases := []struct {
		in, client, desc string
		total, qty       int64
		unit             string
	}{
		{"Invoice Kostas €120", "Kostas", "Services", 12000, 1000, "service"},
		{"Invoice Kostas, 3 hours at €40", "Kostas", "3 hours at €40", 12000, 3000, "hour"},
		{"Invoice Kostas €40.50", "Kostas", "Services", 4050, 1000, "service"},
		{"Invoice Kostas 40,50", "Kostas", "Services", 4050, 1000, "service"},
		{"Invoice Kostas €1,200", "Kostas", "Services", 120000, 1000, "service"},
		{"Invoice Maria Savva €300", "Maria Savva", "Services", 30000, 1000, "service"},
		{"Invoice Kostas 1.5 hours at €40", "Kostas", "1.5 hours at €40", 6000, 1500, "hour"},
		{"bill Elena's Bakery €450 for menu and logo design", "Elena's Bakery", "Menu and logo design", 45000, 1000, "service"},
		{"Invoice Kostas for logo work €200", "Kostas", "Logo work", 20000, 1000, "service"},
		{"Invoice Andreas 2 days at €300 for wiring", "Andreas", "wiring", 60000, 2000, "day"},
		{"charge Nikos: 1 hour @ 50", "Nikos", "1 hour at €50", 5000, 1000, "hour"},
	}
	for _, c := range cases {
		d, missing, ok := InvoiceCommand(c.in)
		if !ok || missing != "" {
			t.Fatalf("%q: ok=%v missing=%q", c.in, ok, missing)
		}
		if d.Client != c.client || d.TotalCents() != c.total || len(d.Lines) != 1 ||
			d.Lines[0].Description != c.desc || d.Lines[0].QuantityMilli != c.qty || d.Lines[0].Unit != c.unit {
			t.Errorf("%q: got %+v (total %d)", c.in, d, d.TotalCents())
		}
	}
}

func TestInvoiceCommandMissing(t *testing.T) {
	for in, want := range map[string]string{"Invoice Kostas": "amount", "Invoice €120": "client", "invoice": "client"} {
		if _, missing, ok := InvoiceCommand(in); !ok || missing != want {
			t.Errorf("%q: missing=%q ok=%v, want %q", in, missing, ok, want)
		}
	}
	for _, in := range []string{"", "   ", "hello", "invoices are great"} {
		if _, _, ok := InvoiceCommand(in); ok {
			t.Errorf("%q should not be an invoice command", in)
		}
	}
}

func TestLineQuantity(t *testing.T) {
	for milli, want := range map[int64]string{3000: "3", 1500: "1.5", 1250: "1.25", 1: "0.001"} {
		if got := (Line{QuantityMilli: milli}).Quantity(); got != want {
			t.Errorf("Quantity(%d) = %q, want %q", milli, got, want)
		}
	}
	if n := (Line{QuantityMilli: 1500, UnitPriceCents: 333}).NetCents(); n != 500 { // 499.5 rounds half up
		t.Errorf("NetCents = %d, want 500", n)
	}
}

func TestDetect(t *testing.T) {
	cases := []struct {
		in     string
		intent Intent
		check  func(Result) bool
	}{
		{"Invoice Kostas €120", CreateInvoice, func(r Result) bool { return r.Draft.Client == "Kostas" && r.Missing == "" }},
		{"invoice kostas", CreateInvoice, func(r Result) bool { return r.Missing == "amount" }},
		{"kostas@example.com", ProvideDetails, func(r Result) bool { return r.Email == "kostas@example.com" }},
		{"his email is nikos@mail.cy", ProvideDetails, func(r Result) bool { return r.Email == "nikos@mail.cy" }},
		{"Remind Andreas", Remind, func(r Result) bool { return r.Client == "Andreas" && !r.All }},
		{"remind everyone", Remind, func(r Result) bool { return r.All }},
		{"Get €800 now", Financing, nil},
		{"Can I get an advance?", Financing, nil},
		{"Who owes me money?", Debtors, nil},
		{"How's my cash looking?", Forecast, nil},
		{"Will I go negative?", Forecast, nil},
		{"hello there", Unknown, nil},
		{"", Unknown, nil},
	}
	for _, c := range cases {
		r := Detect(c.in)
		if r.Intent != c.intent || (c.check != nil && !c.check(r)) {
			t.Errorf("%q: got %+v, want intent %s", c.in, r, c.intent)
		}
	}
}
