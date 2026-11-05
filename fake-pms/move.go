package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/disciplinedware/acs-openclaw-demo/fake-pms/internal/bookings"
)

// bookingsMove is the command bookings-move: it moves the demonstration
// bookings in time and replaces the output file whole, so a running fake PMS
// re-reads either the old file or the new one.
func bookingsMove(args []string) int {
	fs := flag.NewFlagSet("bookings-move", flag.ContinueOnError)
	file := fs.String("file", "/data/bookings.json", "the bookings file to read")
	out := fs.String("out", "", "the file to write (default: -file)")
	ref := fs.String("ref", "", "the reservation that arrives on -day")
	day := fs.String("day", "", "its arrival day, YYYY-MM-DD (default: today in its listing's time zone)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *ref == "" {
		fmt.Fprintln(os.Stderr, "bookings-move: -ref is required")
		return 2
	}
	if *out == "" {
		*out = *file
	}
	var arrival string
	raw, err := os.ReadFile(*file)
	if err == nil {
		raw, arrival, err = bookings.MoveDates(raw, *ref, *day, time.Now())
	}
	if err == nil {
		err = replaceFile(*out, raw)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "bookings-move: %s is unchanged: %v\n", *out, err)
		return 1
	}
	fmt.Printf("bookings-move: %s moved: %s arrives on %s\n", *out, *ref, arrival)
	return 0
}

func replaceFile(path string, raw []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".bookings-*.json")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
