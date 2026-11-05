package bookings

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const listing = `{"id": "l1", "name": "Casa", "time_zone": "America/Cancun", "check_in_time": "15:00", "check_out_time": "11:00"}`

func book(reservation string) string {
	return `{"business_name": "B", "auto_reply": {"guests": true, "others": false}, "access_lead_hours": 2,
		"listings": [` + listing + `], "extras": [], "reservations": [` + reservation + `]}`
}

func TestParseRefuses(t *testing.T) {
	cases := []struct {
		name    string
		file    string
		wantErr string
	}{
		{name: "a_second_document", file: book(``) + ` {"business_name": "C"}`, wantErr: "holds more than one JSON value"},
		{name: "unknown_field", file: `{"business_name": "B", "colour": "red"}`, wantErr: `unknown field "colour"`},
		{name: "dangling_listing", file: book(`{"reservation_ref": "R1", "listing_id": "nope", "kind": "stay", "status": "confirmed", "guest_phone": "+447700900100", "check_in": "2026-11-04", "check_out": "2026-11-05"}`), wantErr: "names no listing"},
		{name: "phone_without_country_code", file: book(`{"reservation_ref": "R1", "listing_id": "l1", "kind": "stay", "status": "confirmed", "guest_phone": "07700900100", "check_in": "2026-11-04", "check_out": "2026-11-05"}`), wantErr: "no E.164 number"},
		{name: "check_out_before_check_in", file: book(`{"reservation_ref": "R1", "listing_id": "l1", "kind": "stay", "status": "confirmed", "guest_phone": "+447700900100", "check_in": "2026-11-06", "check_out": "2026-11-06"}`), wantErr: "check-out is not after check-in"},
		{name: "dates_missing", file: book(`{"reservation_ref": "R1", "listing_id": "l1", "kind": "stay", "status": "confirmed", "guest_phone": "+447700900100"}`), wantErr: "check_in and check_out are required"},
		{name: "unknown_status", file: book(`{"reservation_ref": "R1", "listing_id": "l1", "kind": "stay", "status": "booked", "guest_phone": "+447700900100", "check_in": "2026-11-04", "check_out": "2026-11-05"}`), wantErr: `status "booked"`},
		{name: "block_with_a_guest", file: book(`{"reservation_ref": "R1", "listing_id": "l1", "kind": "block", "status": "confirmed", "guest_phone": "+447700900100", "check_in": "2026-11-04", "check_out": "2026-11-05"}`), wantErr: "a block has no guest"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.file), func(string) string { return "" })
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestTheShippedFileLoads(t *testing.T) {
	b, err := Load("../../data/bookings.json", func(string) string { return "" })
	require.NoError(t, err)
	assert.Equal(t, "Casa Coral", b.BusinessName)
}
