// Package bookings holds the bookings file (bookings.json): one business, its
// listings, extras and reservations, with calendar dates as a booking system
// exports them.
package bookings

import (
	"encoding/json"
	"fmt"
	"time"
)

// Book is the whole bookings file.
type Book struct {
	BusinessName string `json:"business_name"`
	// AutoReply holds the two switches the Guardian reads: whether a routine
	// answer to a guest with a stay, and to everyone else, goes out without
	// the host's approval.
	AutoReply AutoReply `json:"auto_reply"`
	// AccessLeadHours is how many hours before check-in time a paid guest may
	// be given the door code and Wi-Fi.
	AccessLeadHours int           `json:"access_lead_hours"`
	Listings        []Listing     `json:"listings"`
	Extras          []Extra       `json:"extras"`
	Reservations    []Reservation `json:"reservations"`
}

type AutoReply struct {
	Guests bool `json:"guests"`
	Others bool `json:"others"`
}

type Listing struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Address        string  `json:"address"`
	Latitude       float64 `json:"latitude"`
	Longitude      float64 `json:"longitude"`
	City           string  `json:"city"`
	State          string  `json:"state"`
	CountryCode    string  `json:"country_code"`
	TimeZone       string  `json:"time_zone"`
	PersonCapacity int     `json:"person_capacity"`
	CheckInTime    string  `json:"check_in_time"`
	CheckOutTime   string  `json:"check_out_time"`
	GuestGuide     string  `json:"guest_guide"`
	DoorCode       string  `json:"door_code"`
	WifiName       string  `json:"wifi_name"`
	WifiPassword   string  `json:"wifi_password"`

	location *time.Location
}

// Location is the listing's time zone, resolved when the file loads.
func (l *Listing) Location() *time.Location { return l.location }

type Extra struct {
	Code        string      `json:"code"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Price       json.Number `json:"price"`
	Currency    string      `json:"currency"`
}

// Kind of a reservation: a guest's stay, or a block that closes the listing.
const (
	KindStay  = "stay"
	KindBlock = "block"
)

// Lifecycle status of a reservation.
const (
	StatusConfirmed  = "confirmed"
	StatusCheckedIn  = "checked_in"
	StatusCheckedOut = "checked_out"
	StatusCancelled  = "cancelled"
)

type Reservation struct {
	ReservationRef       string `json:"reservation_ref"`
	ChannelReservationID string `json:"channel_reservation_id,omitempty"`
	ListingID            string `json:"listing_id"`
	Kind                 string `json:"kind"`
	Status               string `json:"status"`
	GuestName            string `json:"guest_name,omitempty"`
	GuestPhone           string `json:"guest_phone,omitempty"`
	// GuestPhoneEnv names an environment variable whose value, when set,
	// replaces GuestPhone: the demo's own guest phone, kept out of the file.
	GuestPhoneEnv string      `json:"guest_phone_env,omitempty"`
	GuestLanguage string      `json:"guest_language,omitempty"`
	Adults        int         `json:"adults,omitempty"`
	ChildAges     []int       `json:"child_ages,omitempty"`
	CheckIn       Date        `json:"check_in"`
	CheckOut      Date        `json:"check_out"`
	TotalPrice    json.Number `json:"total_price,omitempty"`
	Currency      string      `json:"currency,omitempty"`
	IsPaid        bool        `json:"is_paid"`
}

// DateLayout is the form of a calendar date in the file: YYYY-MM-DD.
const DateLayout = "2006-01-02"

// Date is a calendar date with no time zone; In places it in a listing's zone.
type Date struct{ t time.Time }

func (d *Date) UnmarshalJSON(raw []byte) error {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return fmt.Errorf("a date is a YYYY-MM-DD string: %w", err)
	}
	t, err := time.Parse(DateLayout, s)
	if err != nil {
		return fmt.Errorf("date %q is not YYYY-MM-DD", s)
	}
	d.t = t
	return nil
}

// IsZero reports a date the file did not give.
func (d Date) IsZero() bool { return d.t.IsZero() }

// In is midnight of the date in loc.
func (d Date) In(loc *time.Location) time.Time {
	return time.Date(d.t.Year(), d.t.Month(), d.t.Day(), 0, 0, 0, 0, loc)
}
