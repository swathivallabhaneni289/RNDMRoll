package spin

import (
	"strings"
	"time"
	// The zone database is compiled into the binary, so a host with no
	// zoneinfo files (or an old one) still knows every zone.
	_ "time/tzdata"
)

const maxZoneNameLength = 64

// ParseZone checks a time zone name the phone sent and loads it. A name is
// accepted only when it is made of letters, digits, underscore, plus, minus
// and slash, is 1 to 64 characters, and every slash-separated part starts
// with a capital letter ("America/New_York"). The capital rule matters on a
// host with a case-blind file system, where "america/new_york" would
// otherwise load and be stored in a spelling other hosts cannot read.
// "Local" is refused: it means this server's own zone.
//
// A name that fails any rule, or is not in the zone database, is
// ErrZoneInvalid.
func ParseZone(name string) (*time.Location, error) {
	if !zoneNameShapeOK(name) {
		return nil, ErrZoneInvalid
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, ErrZoneInvalid
	}
	return loc, nil
}

// loadStoredZone loads a zone name read back from storage. It applies the
// same rules as ParseZone but reports a failure as ErrZoneMissing, so the
// phone is told to report its zone again.
func loadStoredZone(name string) (*time.Location, error) {
	loc, err := ParseZone(name)
	if err != nil {
		return nil, ErrZoneMissing
	}
	return loc, nil
}

func zoneNameShapeOK(name string) bool {
	if name == "" || len(name) > maxZoneNameLength || name == "Local" {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '_', r == '+', r == '-', r == '/':
		default:
			return false
		}
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part[0] < 'A' || part[0] > 'Z' {
			return false
		}
	}
	return true
}
