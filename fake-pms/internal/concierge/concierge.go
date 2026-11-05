// Package concierge answers the agent's guest tools and the Guardian's
// permissions read from the bookings file, at the moment the clock gives.
package concierge

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/disciplinedware/acs-openclaw-demo/fake-pms/internal/bookings"
	"github.com/disciplinedware/acs-openclaw-demo/fake-pms/internal/phone"
)

// Concierge reads the bookings file that books gives at each call. It keeps
// no state: every answer is computed from one read of the file and the clock.
type Concierge struct {
	books func() *bookings.Book
	now   func() time.Time
}

func New(books func() *bookings.Book, now func() time.Time) *Concierge {
	return &Concierge{books: books, now: now}
}

// stay is one reservation with its dates resolved for today.
type stay struct {
	*bookings.Reservation
	listing  *bookings.Listing
	checkIn  time.Time // midnight of the check-in date, in the listing's zone
	checkOut time.Time // midnight of the check-out date, in the listing's zone
}

func (s stay) checkInDate() string  { return s.checkIn.Format(bookings.DateLayout) }
func (s stay) checkOutDate() string { return s.checkOut.Format(bookings.DateLayout) }

// current returns the current stays (confirmed or checked in, check-out today
// or later in the listing's zone), nearest check-in first, then by reference.
func (c *Concierge) current(book *bookings.Book) []stay {
	now := c.now()
	var out []stay
	for i := range book.Reservations {
		r := &book.Reservations[i]
		if r.Kind != bookings.KindStay || (r.Status != bookings.StatusConfirmed && r.Status != bookings.StatusCheckedIn) {
			continue
		}
		l, _ := book.Listing(r.ListingID)
		local := now.In(l.Location())
		today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, l.Location())
		s := stay{
			Reservation: r, listing: l,
			checkIn:  r.CheckIn.In(l.Location()),
			checkOut: r.CheckOut.In(l.Location()),
		}
		if s.checkOut.Before(today) {
			continue
		}
		out = append(out, s)
	}
	slices.SortFunc(out, func(a, b stay) int {
		return cmp.Or(a.checkIn.Compare(b.checkIn), cmp.Compare(a.ReservationRef, b.ReservationRef))
	})
	return out
}

func (c *Concierge) currentOf(book *bookings.Book, guestPhone string) []stay {
	var out []stay
	for _, s := range c.current(book) {
		if s.GuestPhone == guestPhone {
			out = append(out, s)
		}
	}
	return out
}

// accessOpen tells whether a paid stay's guest may be given the door code and
// Wi-Fi now: from access_lead_hours before check-in time on the check-in date
// until check-out time on the check-out date, in the listing's zone.
func (c *Concierge) accessOpen(book *bookings.Book, s stay) bool {
	if !s.IsPaid {
		return false
	}
	from := at(s.checkIn, s.listing.CheckInTime).Add(-time.Duration(book.AccessLeadHours) * time.Hour)
	to := at(s.checkOut, s.listing.CheckOutTime)
	now := c.now()
	return !now.Before(from) && now.Before(to)
}

func at(day time.Time, hhmm string) time.Time {
	var h, m int
	_, _ = fmt.Sscanf(hhmm, "%d:%d", &h, &m)
	return time.Date(day.Year(), day.Month(), day.Day(), h, m, 0, 0, day.Location())
}

func normalizedPhone(raw string) (string, error) {
	p, ok := phone.Normalize(raw)
	if !ok {
		return "", InvalidArgument(fmt.Sprintf("guest_phone %q is not a phone number with its + country code", raw))
	}
	return p, nil
}

// Reservation is one row of pms.reservation.find_by_phone and find_by_ref.
type Reservation struct {
	ReservationRef       string      `json:"reservation_ref"`
	ChannelReservationID string      `json:"channel_reservation_id,omitempty"`
	Status               string      `json:"status"`
	BusinessName         string      `json:"business_name"`
	ListingID            string      `json:"listing_id"`
	ListingName          string      `json:"listing_name"`
	GuestName            string      `json:"guest_name"`
	GuestPhone           string      `json:"guest_phone"`
	GuestLanguage        string      `json:"guest_language,omitempty"`
	Adults               int         `json:"adults,omitempty"`
	ChildAges            []int       `json:"child_ages,omitempty"`
	CheckInDate          string      `json:"check_in_date"`
	CheckOutDate         string      `json:"check_out_date"`
	TotalPrice           json.Number `json:"total_price,omitempty"`
	Currency             string      `json:"currency,omitempty"`
	IsPaid               bool        `json:"is_paid"`
	CheckInTime          string      `json:"check_in_time"`
	CheckOutTime         string      `json:"check_out_time"`
}

type Reservations struct {
	Reservations []Reservation `json:"reservations"`
}

// FindByPhone is pms.reservation.find_by_phone: every current
// reservation of this phone.
func (c *Concierge) FindByPhone(guestPhone string) (*Reservations, error) {
	p, err := normalizedPhone(guestPhone)
	if err != nil {
		return nil, err
	}
	book := c.books()
	found := c.currentOf(book, p)
	if len(found) == 0 {
		return nil, errReservationNotFound
	}
	out := &Reservations{Reservations: []Reservation{}}
	for _, s := range found {
		out.Reservations = append(out.Reservations, reservationOf(book, s))
	}
	return out, nil
}

// FindByRef is pms.reservation.find_by_ref: the current reservation with this
// reservation_ref or channel_reservation_id. It answers whoever asks.
func (c *Concierge) FindByRef(ref string) (*Reservations, error) {
	want := normalizedRef(ref)
	book := c.books()
	for _, s := range c.current(book) {
		if s.ReservationRef == want || (s.ChannelReservationID != "" && normalizedRef(s.ChannelReservationID) == want) {
			return &Reservations{Reservations: []Reservation{reservationOf(book, s)}}, nil
		}
	}
	return nil, errReservationNotFound
}

func reservationOf(book *bookings.Book, s stay) Reservation {
	return Reservation{
		ReservationRef: s.ReservationRef, ChannelReservationID: s.ChannelReservationID, Status: s.Status,
		BusinessName: book.BusinessName, ListingID: s.listing.ID, ListingName: s.listing.Name,
		GuestName: s.GuestName, GuestPhone: s.GuestPhone, GuestLanguage: s.GuestLanguage, Adults: s.Adults, ChildAges: s.ChildAges,
		CheckInDate: s.checkInDate(), CheckOutDate: s.checkOutDate(),
		TotalPrice: s.TotalPrice, Currency: s.Currency, IsPaid: s.IsPaid,
		CheckInTime: s.listing.CheckInTime, CheckOutTime: s.listing.CheckOutTime,
	}
}

// HostReservation is one row of pms.host.list_reservations.
type HostReservation struct {
	ReservationRef string `json:"reservation_ref"`
	GuestName      string `json:"guest_name"`
	GuestPhone     string `json:"guest_phone"`
	ListingName    string `json:"listing_name"`
	CheckInDate    string `json:"check_in_date"`
	CheckOutDate   string `json:"check_out_date"`
}

type HostReservations struct {
	Reservations []HostReservation `json:"reservations"`
}

// ListReservations is pms.host.list_reservations: every current reservation.
func (c *Concierge) ListReservations() *HostReservations {
	out := &HostReservations{Reservations: []HostReservation{}}
	for _, s := range c.current(c.books()) {
		out.Reservations = append(out.Reservations, HostReservation{
			ReservationRef: s.ReservationRef, GuestName: s.GuestName, GuestPhone: s.GuestPhone,
			ListingName: s.listing.Name, CheckInDate: s.checkInDate(), CheckOutDate: s.checkOutDate(),
		})
	}
	return out
}

// Guide is pms.listing.get_guide: a listing's public information.
type Guide struct {
	ListingID      string   `json:"listing_id"`
	BusinessName   string   `json:"business_name"`
	ListingName    string   `json:"listing_name"`
	Address        string   `json:"address"`
	Latitude       float64  `json:"latitude"`
	Longitude      float64  `json:"longitude"`
	City           string   `json:"city"`
	State          string   `json:"state"`
	CountryCode    string   `json:"country_code"`
	TimeZone       string   `json:"time_zone"`
	PersonCapacity int      `json:"person_capacity"`
	CheckInTime    string   `json:"check_in_time"`
	CheckOutTime   string   `json:"check_out_time"`
	GuestGuide     string   `json:"guest_guide"`
	Photos         []string `json:"photos"`
}

func (c *Concierge) GetGuide(listingID string) (*Guide, error) {
	book := c.books()
	l, ok := book.Listing(listingID)
	if !ok {
		return nil, errListingNotFound
	}
	return &Guide{
		ListingID: l.ID, BusinessName: book.BusinessName, ListingName: l.Name, Address: l.Address,
		Latitude: l.Latitude, Longitude: l.Longitude, City: l.City, State: l.State, CountryCode: l.CountryCode,
		TimeZone: l.TimeZone, PersonCapacity: l.PersonCapacity, CheckInTime: l.CheckInTime, CheckOutTime: l.CheckOutTime,
		GuestGuide: l.GuestGuide, Photos: []string{},
	}, nil
}

// Access is pms.listing.get_access: the door code and Wi-Fi of a house.
type Access struct {
	ListingName  string `json:"listing_name"`
	DoorCode     string `json:"door_code"`
	WifiName     string `json:"wifi_name"`
	WifiPassword string `json:"wifi_password"`
}

// GetAccess gives a house's access data while a paid current reservation of
// that house is inside its access window. It does not ask who wants them: the
// Guardian decides that.
func (c *Concierge) GetAccess(listingID string) (*Access, error) {
	book := c.books()
	l, ok := book.Listing(listingID)
	if !ok {
		return nil, errListingNotFound
	}
	for _, s := range c.current(book) {
		if s.listing.ID == l.ID && c.accessOpen(book, s) {
			return &Access{ListingName: l.Name, DoorCode: l.DoorCode, WifiName: l.WifiName, WifiPassword: l.WifiPassword}, nil
		}
	}
	return nil, errAccessNotAvailable
}

// ExtraRow is one row of pms.extra.list.
type ExtraRow struct {
	Code        string      `json:"code"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Price       json.Number `json:"price"`
	Currency    string      `json:"currency"`
}

func (c *Concierge) ListExtras() []ExtraRow {
	out := []ExtraRow{}
	for _, e := range c.books().Extras {
		out = append(out, ExtraRow(e))
	}
	slices.SortFunc(out, func(a, b ExtraRow) int { return cmp.Compare(a.Name, b.Name) })
	return out
}

// Permissions is the Guardian's live read: the auto-reply switches and, for
// each asked phone, every current reference and secret of the business,
// marked for that guest.
type Permissions struct {
	AutoReply bookings.AutoReply          `json:"auto_reply"`
	Guests    map[string]GuestPermissions `json:"guests"`
}

type GuestPermissions struct {
	References []Reference `json:"references"`
	Secrets    []Secret    `json:"secrets"`
	// Stay is the guest's nearest current stay; absent when there is none.
	Stay *Stay `json:"stay,omitempty"`
}

type Reference struct {
	Value string `json:"value"`
	Own   bool   `json:"own"`
}

type Secret struct {
	Value   string `json:"value"`
	Allowed bool   `json:"allowed"`
}

type Stay struct {
	ReservationRef string `json:"reservation_ref"`
	GuestName      string `json:"guest_name"`
	ListingName    string `json:"listing_name"`
	CheckInDate    string `json:"check_in_date"`
	CheckOutDate   string `json:"check_out_date"`
}

// GuestPermissions answers for each phone as given (the Guardian sends E.164).
func (c *Concierge) GuestPermissions(guestPhones []string) *Permissions {
	book := c.books()
	current := c.current(book)
	out := &Permissions{AutoReply: book.AutoReply, Guests: map[string]GuestPermissions{}}
	for _, p := range guestPhones {
		allowed := map[string]bool{}
		for _, s := range current {
			if s.GuestPhone == p && c.accessOpen(book, s) {
				allowed[s.listing.ID] = true
			}
		}
		g := GuestPermissions{References: []Reference{}, Secrets: []Secret{}}
		for _, s := range current {
			for _, v := range []string{s.ReservationRef, s.ChannelReservationID} {
				if v != "" {
					g.References = append(g.References, Reference{Value: v, Own: s.GuestPhone == p})
				}
			}
			if g.Stay == nil && s.GuestPhone == p {
				g.Stay = &Stay{
					ReservationRef: s.ReservationRef, GuestName: s.GuestName, ListingName: s.listing.Name,
					CheckInDate: s.checkInDate(), CheckOutDate: s.checkOutDate(),
				}
			}
		}
		for _, l := range book.Listings {
			for _, v := range []string{l.DoorCode, l.WifiPassword} {
				if v != "" {
					g.Secrets = append(g.Secrets, Secret{Value: v, Allowed: allowed[l.ID]})
				}
			}
		}
		out.Guests[p] = g
	}
	return out
}

// normalizedRef is a reference as the guest may write it: any case, padded.
func normalizedRef(ref string) string { return strings.ToUpper(strings.TrimSpace(ref)) }
