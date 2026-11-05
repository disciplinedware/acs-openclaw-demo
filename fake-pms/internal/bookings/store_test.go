package bookings

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

const stay = `{"reservation_ref": "R1", "listing_id": "l1", "kind": "stay", "status": "confirmed", "guest_phone": "+447700900100", "check_in": "2026-11-04", "check_out": "2026-11-06"}`

// replace writes the file as a writer must: a temporary file, then a rename.
func replace(t *testing.T, path, content string, modTime time.Time) {
	t.Helper()
	tmp := path + ".tmp"
	require.NoError(t, os.WriteFile(tmp, []byte(content), 0o600))
	require.NoError(t, os.Chtimes(tmp, modTime, modTime))
	require.NoError(t, os.Rename(tmp, path))
}

func TestStoreReloadsAChangedFileAndKeepsTheLastValidOne(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bookings.json")
	start := time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC)
	replace(t, path, book(stay), start)
	core, logs := observer.New(zap.InfoLevel)
	s, err := OpenStore(path, func(string) string { return "" }, zap.New(core))
	require.NoError(t, err)
	require.Len(t, s.Book().Reservations, 1)

	replace(t, path, book(stay+`, {"reservation_ref": "R2", "listing_id": "l1", "kind": "block", "status": "confirmed", "check_in": "2026-11-07", "check_out": "2026-11-08"}`), start.Add(time.Minute))
	s.reload()
	assert.Len(t, s.Book().Reservations, 2, "a valid change is served")

	replace(t, path, book(`{"reservation_ref": "R3", "listing_id": "l1", "kind": "stay", "status": "confirmed", "guest_phone": "+447700900100", "check_in": "2026-11-08", "check_out": "2026-11-07"}`), start.Add(2*time.Minute))
	s.reload()
	assert.Len(t, s.Book().Reservations, 2, "an invalid file is refused and the last valid data stays")
	refused := logs.FilterMessage("bookings file refused; the last valid data stays").All()
	require.Len(t, refused, 1)
	assert.Contains(t, refused[0].ContextMap()["error"], "check-out is not after check-in")

	s.reload()
	assert.Len(t, logs.FilterMessage("bookings file refused; the last valid data stays").All(), 1, "an unchanged file is not read again")
}

func TestOpenStoreRefusesAnInvalidFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bookings.json")
	replace(t, path, book(`{"reservation_ref": "R1", "listing_id": "l1", "kind": "stay", "status": "confirmed", "guest_phone": "+447700900100", "check_in": "4 Nov", "check_out": "2026-11-06"}`), time.Now())
	_, err := OpenStore(path, func(string) string { return "" }, zap.NewNop())
	require.Error(t, err)
	assert.Contains(t, err.Error(), `date "4 Nov" is not YYYY-MM-DD`)
}
