package bookings

import (
	"fmt"
	"regexp"
	"time"
)

var reservationDate = regexp.MustCompile(`("(?:check_in|check_out)"\s*:\s*")(\d{4}-\d{2}-\d{2})"`)

// MoveDates shifts every reservation's check_in and check_out by the same
// number of days, so that reservation ref arrives on day (YYYY-MM-DD; empty
// is today in that reservation's listing time zone). The calendar keeps its
// order and gaps, and the rest of the file keeps its bytes. It returns the
// moved file and the arrival day.
func MoveDates(raw []byte, ref, day string, now time.Time) ([]byte, string, error) {
	noEnv := func(string) string { return "" }
	b, err := Parse(raw, noEnv)
	if err != nil {
		return nil, "", err
	}
	var anchor *Reservation
	for i := range b.Reservations {
		if b.Reservations[i].ReservationRef == ref {
			anchor = &b.Reservations[i]
		}
	}
	if anchor == nil {
		return nil, "", fmt.Errorf("the bookings file has no reservation %q", ref)
	}
	target, err := arrivalDay(b, anchor, day, now)
	if err != nil {
		return nil, "", err
	}
	days := int(target.Sub(anchor.CheckIn.In(time.UTC)).Hours() / 24)

	moved := 0
	var moveErr error
	out := reservationDate.ReplaceAllFunc(raw, func(match []byte) []byte {
		parts := reservationDate.FindSubmatch(match)
		d, err := time.Parse(DateLayout, string(parts[2]))
		if err != nil {
			moveErr = fmt.Errorf("date %q is not YYYY-MM-DD", parts[2])
			return match
		}
		moved++
		return []byte(string(parts[1]) + d.AddDate(0, 0, days).Format(DateLayout) + `"`)
	})
	if moveErr != nil {
		return nil, "", moveErr
	}
	if want := 2 * len(b.Reservations); moved != want {
		return nil, "", fmt.Errorf("moved %d dates, the reservations hold %d: write each check_in and check_out as \"key\": \"YYYY-MM-DD\"", moved, want)
	}
	if _, err := Parse(out, noEnv); err != nil {
		return nil, "", fmt.Errorf("the moved file: %w", err)
	}
	return out, target.Format(DateLayout), nil
}

func arrivalDay(b *Book, anchor *Reservation, day string, now time.Time) (time.Time, error) {
	if day != "" {
		t, err := time.Parse(DateLayout, day)
		if err != nil {
			return time.Time{}, fmt.Errorf("day %q is not YYYY-MM-DD", day)
		}
		return t, nil
	}
	l, _ := b.Listing(anchor.ListingID)
	local := now.In(l.Location())
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC), nil
}
