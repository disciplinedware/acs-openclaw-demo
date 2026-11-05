package bookings

import (
	"context"
	"os"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// Store holds the last valid bookings file and re-reads the file when its
// size or modification time changes. A file that fails validation is refused:
// the last valid data stays and the error is logged. A writer replaces the
// file whole (a temporary file, then a rename), so a read never sees half of it.
type Store struct {
	path   string
	getenv func(string) string
	log    *zap.Logger
	book   atomic.Pointer[Book]
	seen   fileVersion
}

type fileVersion struct {
	size    int64
	modTime time.Time
}

// OpenStore loads the file; a file that is invalid at start is an error.
func OpenStore(path string, getenv func(string) string, log *zap.Logger) (*Store, error) {
	s := &Store{path: path, getenv: getenv, log: log}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	b, err := Load(path, getenv)
	if err != nil {
		return nil, err
	}
	s.book.Store(b)
	s.seen = versionOf(info)
	return s, nil
}

// Book is the last valid bookings file.
func (s *Store) Book() *Book { return s.book.Load() }

// Watch checks the file every interval until ctx ends.
func (s *Store) Watch(ctx context.Context, interval time.Duration) {
	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			s.reload()
		}
	}
}

// reload reads the file once if it changed since the last read.
func (s *Store) reload() {
	info, err := os.Stat(s.path)
	if err != nil {
		if s.seen != (fileVersion{}) {
			s.log.Error("bookings file cannot be read; the last valid data stays", zap.String("file", s.path), zap.Error(err))
			s.seen = fileVersion{}
		}
		return
	}
	v := versionOf(info)
	if v == s.seen {
		return
	}
	s.seen = v
	b, err := Load(s.path, s.getenv)
	if err != nil {
		s.log.Error("bookings file refused; the last valid data stays", zap.String("file", s.path), zap.Error(err))
		return
	}
	s.book.Store(b)
	s.log.Info("bookings file reloaded", zap.String("file", s.path), zap.Int("reservations", len(b.Reservations)))
}

func versionOf(info os.FileInfo) fileVersion {
	return fileVersion{size: info.Size(), modTime: info.ModTime()}
}
