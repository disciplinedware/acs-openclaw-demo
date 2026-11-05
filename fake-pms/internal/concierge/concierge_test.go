package concierge

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/disciplinedware/acs-openclaw-demo/fake-pms/internal/bookings"
)

const (
	maria    = "+447700900100" // CC-1042 at Casa Coral, arrives today, paid
	lopez    = "+447700900110" // CC-1045 at Casita Coral, arrives tomorrow
	stranger = "+447700900199"
)

var cancun = mustZone("America/Cancun")

func mustZone(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

// onDemoDay is a moment on 2026-11-04 in Cancún, the day the test bookings make
// CC-1042 arrive.
func onDemoDay(hour, minute int) time.Time {
	return time.Date(2026, 11, 4, hour, minute, 0, 0, cancun)
}

func newConcierge(t *testing.T, now time.Time, env map[string]string) *Concierge {
	t.Helper()
	book, err := bookings.Load("../../testdata/bookings.json", func(k string) string { return env[k] })
	require.NoError(t, err)
	return New(func() *bookings.Book { return book }, func() time.Time { return now })
}

func problemCode(t *testing.T, err error) string {
	t.Helper()
	p, ok := err.(*Problem)
	require.True(t, ok, "error %v is no Problem", err)
	return p.Type
}

func TestFindByRef(t *testing.T) {
	c := newConcierge(t, onDemoDay(9, 0), nil)
	for _, ref := range []string{"CC-1042", " cc-1042 ", "BK-4410021"} {
		got, err := c.FindByRef(ref)
		require.NoError(t, err, ref)
		require.Len(t, got.Reservations, 1)
		assert.Equal(t, "CC-1042", got.Reservations[0].ReservationRef, ref)
	}
	_, err := c.FindByRef("CC-9999")
	var p *Problem
	require.ErrorAs(t, err, &p)
	assert.Equal(t, "/errors/pms.reservation_not_found", p.Type)
}

// The host's calendar: every current stay with its guest and dates; a stay that
// ended and a block (no guest) are not in it.
func TestListReservations(t *testing.T) {
	got := newConcierge(t, onDemoDay(9, 0), nil).ListReservations()
	refs := map[string]HostReservation{}
	for _, r := range got.Reservations {
		refs[r.ReservationRef] = r
	}
	assert.Equal(t, HostReservation{
		ReservationRef: "CC-1042", GuestName: "Maria Gonzalez", GuestPhone: "+447700900100",
		ListingName: "Casa Coral", CheckInDate: "2026-11-04", CheckOutDate: "2026-11-06",
	}, refs["CC-1042"])
	assert.Contains(t, refs, "CC-1053", "a December stay is on the calendar")
	assert.NotContains(t, refs, "CC-1041", "a stay that ended")
	assert.NotContains(t, refs, "CC-1051", "a block")
}

func TestFindByPhone(t *testing.T) {
	cases := []struct {
		name     string
		phone    string
		wantRefs []string
		wantErr  string
	}{
		{name: "stage_guest", phone: maria, wantRefs: []string{"CC-1042"}},
		{name: "spaced_form_is_the_same_guest", phone: "+44 7700 900100", wantRefs: []string{"CC-1042"}},
		{name: "another_guest", phone: lopez, wantRefs: []string{"CC-1045"}},
		{name: "no_current_stay", phone: stranger, wantErr: "/errors/pms.reservation_not_found"},
		{name: "past_stay_is_not_current", phone: "+447700900103", wantErr: "/errors/pms.reservation_not_found"},
		{name: "no_country_code", phone: "07700 900100", wantErr: "/errors/pms.invalid_argument"},
		{name: "not_a_phone", phone: "maria", wantErr: "/errors/pms.invalid_argument"},
	}
	c := newConcierge(t, onDemoDay(9, 0), nil)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := c.FindByPhone(tc.phone)
			if tc.wantErr != "" {
				assert.Equal(t, tc.wantErr, problemCode(t, err))
				return
			}
			require.NoError(t, err)
			var refs []string
			for _, r := range got.Reservations {
				refs = append(refs, r.ReservationRef)
			}
			assert.Equal(t, tc.wantRefs, refs)
		})
	}
}

// The file's calendar dates are served as they are, and a stay stops being
// current after its check-out day.
func TestCalendarDates(t *testing.T) {
	got, err := newConcierge(t, onDemoDay(9, 0), nil).FindByPhone(maria)
	require.NoError(t, err)
	r := got.Reservations[0]
	assert.Equal(t, "2026-11-04", r.CheckInDate)
	assert.Equal(t, "2026-11-06", r.CheckOutDate)
	assert.Equal(t, "Casa Coral", r.ListingName)
	assert.Equal(t, "15:00", r.CheckInTime)

	_, err = newConcierge(t, onDemoDay(9, 0).AddDate(0, 0, 3), nil).FindByPhone(maria)
	assert.Equal(t, "/errors/pms.reservation_not_found", problemCode(t, err))
}

// "Today" is the listing's day, not UTC's: at 21:00 in Cancún on the
// check-out day it is already the next day in UTC, and the stay is current.
func TestTodayIsTheListingsDay(t *testing.T) {
	got, err := newConcierge(t, onDemoDay(21, 0).AddDate(0, 0, 2).UTC(), nil).FindByPhone(maria)
	require.NoError(t, err)
	assert.Equal(t, "2026-11-06", got.Reservations[0].CheckOutDate)
}

func TestGuestPhoneFromTheEnvironment(t *testing.T) {
	c := newConcierge(t, onDemoDay(9, 0), map[string]string{"GUEST_PHONE": "+44 7700 900 198"})
	got, err := c.FindByPhone("+447700900198")
	require.NoError(t, err)
	assert.Equal(t, "CC-1042", got.Reservations[0].ReservationRef)
	_, err = c.FindByPhone(maria)
	assert.Equal(t, "/errors/pms.reservation_not_found", problemCode(t, err))
}

// The window is tested on a reservation that checks out on the demo day and
// on an unpaid one that arrives that day.
const accessBook = `{"business_name": "B", "auto_reply": {"guests": true, "others": true}, "access_lead_hours": 2,
  "listings": [{"id": "l1", "name": "Casa Coral", "time_zone": "America/Cancun", "check_in_time": "15:00", "check_out_time": "11:00",
    "door_code": "4821", "wifi_name": "CasaCoral", "wifi_password": "arrecife-2026"},
    {"id": "l2", "name": "Casita Coral", "time_zone": "America/Cancun", "check_in_time": "15:00", "check_out_time": "11:00",
    "door_code": "7316", "wifi_name": "CasitaCoral", "wifi_password": "arrecife-2026-b"}],
  "extras": [],
  "reservations": [
    {"reservation_ref": "CC-1042", "listing_id": "l1", "kind": "stay", "status": "confirmed", "guest_name": "Maria", "guest_phone": "+447700900100", "check_in": "2026-11-04", "check_out": "2026-11-06", "is_paid": true},
    {"reservation_ref": "CC-2001", "listing_id": "l1", "kind": "stay", "status": "checked_in", "guest_name": "Ana", "guest_phone": "+447700900201", "check_in": "2026-11-01", "check_out": "2026-11-04", "is_paid": true},
    {"reservation_ref": "CC-2002", "listing_id": "l2", "kind": "stay", "status": "confirmed", "guest_name": "Luis", "guest_phone": "+447700900202", "check_in": "2026-11-04", "check_out": "2026-11-05", "is_paid": false},
    {"reservation_ref": "CC-2003", "listing_id": "l2", "kind": "stay", "status": "cancelled", "guest_name": "Eva", "guest_phone": "+447700900203", "check_in": "2026-11-04", "check_out": "2026-11-05", "is_paid": true}
  ]}`

func TestGetAccess(t *testing.T) {
	book, err := bookings.Parse([]byte(accessBook), func(string) string { return "" })
	require.NoError(t, err)
	cases := []struct {
		name    string
		now     time.Time
		listing string
		wantErr string
	}{
		{name: "before_the_window", now: onDemoDay(12, 59), listing: "l1", wantErr: "/errors/pms.listing_access_not_available"},
		{name: "window_opens_two_hours_before_check_in", now: onDemoDay(13, 0), listing: "l1"},
		{name: "last_minute_before_check_out", now: onDemoDay(10, 59), listing: "l1"},
		{name: "window_closes_at_check_out_time", now: onDemoDay(11, 0), listing: "l1", wantErr: "/errors/pms.listing_access_not_available"},
		{name: "a_house_with_only_unpaid_and_cancelled_stays", now: onDemoDay(14, 0), listing: "l2", wantErr: "/errors/pms.listing_access_not_available"},
		{name: "an_unknown_house", now: onDemoDay(14, 0), listing: "l9", wantErr: "/errors/pms.listing_not_found"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := New(func() *bookings.Book { return book }, func() time.Time { return tc.now }).GetAccess(tc.listing)
			if tc.wantErr != "" {
				assert.Equal(t, tc.wantErr, problemCode(t, err))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "4821", got.DoorCode)
			assert.Equal(t, "arrecife-2026", got.WifiPassword)
		})
	}
}

func TestGetGuide(t *testing.T) {
	c := newConcierge(t, onDemoDay(9, 0), nil)
	got, err := c.GetGuide("2f6a9c1e-5b3d-4c8a-9e10-000000000001")
	require.NoError(t, err)
	assert.Equal(t, "Casa Coral", got.ListingName)
	assert.NotContains(t, got.GuestGuide, "4821")
	_, err = c.GetGuide("nope")
	assert.Equal(t, "/errors/pms.listing_not_found", problemCode(t, err))
}

func TestListExtras(t *testing.T) {
	got := newConcierge(t, onDemoDay(9, 0), nil).ListExtras()
	require.Len(t, got, 2)
	assert.Equal(t, "cenote-tour", got[0].Code, "sorted by name: Tour before Traslado")
	assert.Equal(t, "1800", got[0].Price.String())
}

func TestGuestPermissions(t *testing.T) {
	d := newConcierge(t, onDemoDay(14, 0), nil).GuestPermissions([]string{maria, stranger})
	assert.Equal(t, bookings.AutoReply{Guests: false, Others: false}, d.AutoReply)

	own := func(g GuestPermissions) map[string]bool {
		m := map[string]bool{}
		for _, r := range g.References {
			m[r.Value] = r.Own
		}
		return m
	}
	allowed := func(g GuestPermissions) map[string]bool {
		m := map[string]bool{}
		for _, s := range g.Secrets {
			m[s.Value] = s.Allowed
		}
		return m
	}

	m := d.Guests[maria]
	assert.True(t, own(m)["CC-1042"])
	assert.True(t, own(m)["BK-4410021"])
	assert.False(t, own(m)["CC-1045"])
	assert.NotContains(t, own(m), "CC-1041", "a past stay is no current reference")
	assert.NotContains(t, own(m), "CC-1051", "a block is no guest's")
	assert.True(t, allowed(m)["4821"])
	assert.True(t, allowed(m)["arrecife-2026"])
	assert.False(t, allowed(m)["7316"])
	require.NotNil(t, m.Stay)
	assert.Equal(t, Stay{ReservationRef: "CC-1042", GuestName: "Maria Gonzalez", ListingName: "Casa Coral", CheckInDate: "2026-11-04", CheckOutDate: "2026-11-06"}, *m.Stay)

	s := d.Guests[stranger]
	assert.Nil(t, s.Stay)
	for _, r := range s.References {
		assert.False(t, r.Own, r.Value)
	}
	for _, sec := range s.Secrets {
		assert.False(t, sec.Allowed, sec.Value)
	}
}

func TestGuestPermissionsBeforeTheAccessWindow(t *testing.T) {
	d := newConcierge(t, onDemoDay(9, 0), nil).GuestPermissions([]string{maria})
	for _, s := range d.Guests[maria].Secrets {
		assert.False(t, s.Allowed, s.Value)
	}
}
