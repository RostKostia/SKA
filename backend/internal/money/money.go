// Package money formats and parses euro amounts held as integer cents.
package money

import (
	"regexp"
	"strconv"
	"strings"
)

// Format gives "€120", "€40.50", "€1,200" or "−€800".
func Format(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "−"
		cents = -cents
	}
	euros, rest := cents/100, cents%100
	s := group(euros)
	if rest != 0 {
		s += "." + pad2(rest)
	}
	return sign + "€" + s
}

func group(n int64) string {
	s := strconv.FormatInt(n, 10)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

func pad2(n int64) string {
	if n < 10 {
		return "0" + strconv.FormatInt(n, 10)
	}
	return strconv.FormatInt(n, 10)
}

var thousandsComma = regexp.MustCompile(`,\d{3}(\D|$)`)

// ParseCents reads "€40.50", "40,50", "€1,200" or "120" as cents. A comma followed by
// exactly three digits is a thousands separator; otherwise a comma is the decimal point.
func ParseCents(raw string) (int64, bool) {
	var b strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' || r == '.' || r == ',' {
			b.WriteRune(r)
		}
	}
	s := strings.Trim(b.String(), ".,")
	if s == "" {
		return 0, false
	}
	if thousandsComma.MatchString(s) {
		s = strings.ReplaceAll(s, ",", "")
	} else {
		s = strings.Replace(s, ",", ".", 1)
	}
	whole, frac, _ := strings.Cut(s, ".")
	if strings.ContainsAny(frac, ".,") || whole == "" && frac == "" {
		return 0, false
	}
	w, err := strconv.ParseInt(whole, 10, 64)
	if whole != "" && err != nil {
		return 0, false
	}
	frac += "000"
	f, err := strconv.ParseInt(frac[:3], 10, 64) // thousandths, then round to cents
	if err != nil {
		return 0, false
	}
	cents := w*100 + (f+5)/10
	return cents, cents > 0
}
