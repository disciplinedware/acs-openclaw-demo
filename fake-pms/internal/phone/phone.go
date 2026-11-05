// Package phone normalizes a phone parameter to E.164 by the same rule as the
// Guardian's rules (guardian/policy/lib/assistant.rego, normalized_phone), for
// the forms written with their `+` country code: the fake PMS, like a real
// booking system, refuses a number without it.
package phone

import (
	"regexp"
	"strings"
)

var (
	separators    = regexp.MustCompile(`[ .()-]`)
	mexican       = regexp.MustCompile(`^\+52[0-9]{10}$`)
	mexicanOld    = regexp.MustCompile(`^\+521[0-9]{10}$`)
	international = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)
)

// Normalize returns the E.164 form of raw and true, or "" and false when raw
// is no phone. A Mexican mobile in its older +521 form is the same phone as
// its +52 form.
func Normalize(raw string) (string, bool) {
	s := separators.ReplaceAllString(raw, "")
	switch {
	case mexican.MatchString(s):
		return s, true
	case mexicanOld.MatchString(s):
		return "+52" + s[len(s)-10:], true
	case international.MatchString(s) && !strings.HasPrefix(s, "+52"):
		return s, true
	}
	return "", false
}
