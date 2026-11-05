package bookings

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const twoStays = `{"reservation_ref": "R1", "listing_id": "l1", "kind": "stay", "status": "confirmed", "guest_phone": "+447700900100", "check_in": "2026-11-04", "check_out": "2026-11-06"},
	{"reservation_ref": "R2", "listing_id": "l1", "kind": "block", "status": "confirmed", "check_in":"2026-10-30", "check_out": "2026-11-01"}`

func TestMoveDates(t *testing.T) {
	// 2026-11-11 03:00 UTC is still 2026-11-10 in Cancún (UTC-5).
	now := time.Date(2026, 11, 11, 3, 0, 0, 0, time.UTC)
	cases := []struct {
		name, day, arrival string
		want               []string
	}{
		{name: "to_the_given_day", day: "2026-11-11", arrival: "2026-11-11", want: []string{`"check_in": "2026-11-11"`, `"check_out": "2026-11-13"`, `"check_in":"2026-11-06"`, `"check_out": "2026-11-08"`}},
		{name: "back_in_time", day: "2026-11-01", arrival: "2026-11-01", want: []string{`"check_in": "2026-11-01"`, `"check_out": "2026-11-03"`, `"check_in":"2026-10-27"`, `"check_out": "2026-10-29"`}},
		{name: "today_in_the_listing_zone", arrival: "2026-11-10", want: []string{`"check_in": "2026-11-10"`, `"check_out": "2026-11-12"`, `"check_in":"2026-11-05"`, `"check_out": "2026-11-07"`}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := book(twoStays)
			out, arrival, err := MoveDates([]byte(raw), "R1", tc.day, now)
			require.NoError(t, err)
			assert.Equal(t, tc.arrival, arrival)
			for _, w := range tc.want {
				assert.Contains(t, string(out), w)
			}
			assert.Equal(t, len(raw), len(out), "only the dates change")
			assert.Contains(t, string(out), `"check_in_time": "15:00"`)
		})
	}
}

func TestMoveDatesRefuses(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name, file, ref, day, wantErr string
	}{
		{name: "unknown_reservation", file: book(twoStays), ref: "R9", wantErr: `no reservation "R9"`},
		{name: "bad_day", file: book(twoStays), ref: "R1", day: "11/11/2026", wantErr: "is not YYYY-MM-DD"},
		{name: "invalid_file", file: book(`{"reservation_ref": "R1"}`), ref: "R1", wantErr: "listing_id"},
		{name: "a_date_the_pattern_misses", file: book(strings.Replace(twoStays, `"check_out": "2026-11-06"`, `"check\u005fout": "2026-11-06"`, 1)), ref: "R1", wantErr: "moved 3 dates"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := MoveDates([]byte(tc.file), tc.ref, tc.day, now)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestTheShippedFileMoves(t *testing.T) {
	raw, err := os.ReadFile("../../data/bookings.json")
	require.NoError(t, err)
	out, _, err := MoveDates(raw, "CC-1042", "2026-11-11", time.Now())
	require.NoError(t, err)
	b, err := Parse(out, func(string) string { return "" })
	require.NoError(t, err)
	for _, r := range b.Reservations {
		if r.ReservationRef == "CC-1042" {
			assert.Equal(t, "2026-11-11", r.CheckIn.In(time.UTC).Format(DateLayout))
		}
	}
}
