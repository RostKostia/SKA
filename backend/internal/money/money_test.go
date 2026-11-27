package money

import "testing"

func TestFormat(t *testing.T) {
	cases := map[int64]string{12000: "€120", 4050: "€40.50", 120000: "€1,200", -80000: "−€800", 5: "€0.05", 123456789: "€1,234,567.89"}
	for in, want := range cases {
		if got := Format(in); got != want {
			t.Errorf("Format(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestParseCents(t *testing.T) {
	cases := map[string]int64{"€120": 12000, "€40.50": 4050, "40,50": 4050, "€1,200": 120000, "40.5": 4050, "0.005": 1, "€ 30": 3000}
	for in, want := range cases {
		if got, ok := ParseCents(in); !ok || got != want {
			t.Errorf("ParseCents(%q) = %d, %v; want %d", in, got, ok, want)
		}
	}
	for _, bad := range []string{"", "€", "abc", "0", "1.2.3"} {
		if _, ok := ParseCents(bad); ok {
			t.Errorf("ParseCents(%q) should fail", bad)
		}
	}
}
