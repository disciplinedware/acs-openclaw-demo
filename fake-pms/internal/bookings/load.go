package bookings

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/disciplinedware/acs-openclaw-demo/fake-pms/internal/phone"
)

var clock = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

// Load reads and validates the bookings file. getenv resolves each
// reservation's guest_phone_env.
func Load(path string, getenv func(string) string) (*Book, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(raw, getenv)
}

// Parse decodes a bookings file and refuses one the fake PMS cannot serve
// exactly: a second JSON value, an unknown field, a duplicate, a dangling
// listing, a phone that is no E.164 number.
func Parse(raw []byte, getenv func(string) string) (*Book, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var b Book
	if err := dec.Decode(&b); err != nil {
		return nil, fmt.Errorf("bookings file: %w", err)
	}
	if dec.More() {
		return nil, errors.New("bookings file holds more than one JSON value")
	}
	if err := b.resolve(getenv); err != nil {
		return nil, fmt.Errorf("bookings file: %w", err)
	}
	return &b, nil
}

func (b *Book) resolve(getenv func(string) string) error {
	var errs []error
	if b.BusinessName == "" {
		errs = append(errs, errors.New("business_name is empty"))
	}
	if b.AccessLeadHours < 0 {
		errs = append(errs, fmt.Errorf("access_lead_hours is %d; it is zero or more", b.AccessLeadHours))
	}
	listings := map[string]bool{}
	for i := range b.Listings {
		l := &b.Listings[i]
		if l.ID == "" || listings[l.ID] {
			errs = append(errs, fmt.Errorf("listing %q: id is empty or repeated", l.Name))
		}
		listings[l.ID] = true
		loc, err := time.LoadLocation(l.TimeZone)
		if err != nil || l.TimeZone == "" {
			errs = append(errs, fmt.Errorf("listing %q: time_zone %q is no IANA zone", l.Name, l.TimeZone))
		}
		l.location = loc
		if !clock.MatchString(l.CheckInTime) || !clock.MatchString(l.CheckOutTime) {
			errs = append(errs, fmt.Errorf("listing %q: check_in_time and check_out_time are HH:MM", l.Name))
		}
	}
	extras := map[string]bool{}
	for _, e := range b.Extras {
		if e.Code == "" || extras[e.Code] {
			errs = append(errs, fmt.Errorf("extra %q: code is empty or repeated", e.Name))
		}
		extras[e.Code] = true
	}
	refs := map[string]bool{}
	for i := range b.Reservations {
		r := &b.Reservations[i]
		if r.ReservationRef == "" || refs[r.ReservationRef] {
			errs = append(errs, fmt.Errorf("reservation %q: reservation_ref is empty or repeated", r.ReservationRef))
		}
		refs[r.ReservationRef] = true
		if !listings[r.ListingID] {
			errs = append(errs, fmt.Errorf("reservation %s: listing_id %q names no listing", r.ReservationRef, r.ListingID))
		}
		switch r.Status {
		case StatusConfirmed, StatusCheckedIn, StatusCheckedOut, StatusCancelled:
		default:
			errs = append(errs, fmt.Errorf("reservation %s: status %q is not confirmed, checked_in, checked_out or cancelled", r.ReservationRef, r.Status))
		}
		switch {
		case r.CheckIn.IsZero() || r.CheckOut.IsZero():
			errs = append(errs, fmt.Errorf("reservation %s: check_in and check_out are required", r.ReservationRef))
		case !r.CheckOut.In(time.UTC).After(r.CheckIn.In(time.UTC)):
			errs = append(errs, fmt.Errorf("reservation %s: check-out is not after check-in", r.ReservationRef))
		}
		switch r.Kind {
		case KindBlock:
			if r.GuestPhone != "" || r.GuestPhoneEnv != "" {
				errs = append(errs, fmt.Errorf("reservation %s: a block has no guest", r.ReservationRef))
			}
		case KindStay:
			if r.GuestPhoneEnv != "" {
				if v := getenv(r.GuestPhoneEnv); v != "" {
					r.GuestPhone = v
				}
			}
			e164, ok := phone.Normalize(r.GuestPhone)
			if !ok {
				errs = append(errs, fmt.Errorf("reservation %s: guest_phone %q is no E.164 number", r.ReservationRef, r.GuestPhone))
			}
			r.GuestPhone = e164
		default:
			errs = append(errs, fmt.Errorf("reservation %s: kind %q is not stay or block", r.ReservationRef, r.Kind))
		}
	}
	return errors.Join(errs...)
}

// Listing returns the listing with this id.
func (b *Book) Listing(id string) (*Listing, bool) {
	for i := range b.Listings {
		if b.Listings[i].ID == id {
			return &b.Listings[i], true
		}
	}
	return nil, false
}
