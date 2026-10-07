package user

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func utc(y int, m time.Month, d, hh int) time.Time {
	return time.Date(y, m, d, hh, 0, 0, 0, time.UTC)
}

func TestValidateBirthdate_AcceptsOrdinaryDate(t *testing.T) {
	got, err := ValidateBirthdate("1990-01-31", utc(2026, 10, 7, 12))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Year() != 1990 || got.Month() != time.January || got.Day() != 31 {
		t.Fatalf("got %v", got)
	}
}

func TestValidateBirthdate_RejectsNonexistentDates(t *testing.T) {
	now := utc(2026, 10, 7, 12)
	for _, s := range []string{
		"2000-02-30", "2001-02-29", "1999-04-31", "2000-13-01", "2000-00-10",
		"2000-01-00", "2000-1-5", "20000105", "", "not a date", "2000-01-05T00:00:00Z",
		" 2000-01-05", "2000-01-05 ", "+2000-01-05", "12000-01-05",
	} {
		if _, err := ValidateBirthdate(s, now); !errors.Is(err, ErrBirthdateInvalid) {
			t.Errorf("ValidateBirthdate(%q) = %v, want ErrBirthdateInvalid", s, err)
		}
	}
}

func TestValidateBirthdate_AcceptsLeapDay(t *testing.T) {
	if _, err := ValidateBirthdate("2000-02-29", utc(2026, 10, 7, 12)); err != nil {
		t.Fatalf("29 February in a leap year must be valid: %v", err)
	}
}

func TestValidateBirthdate_RejectsBefore1900(t *testing.T) {
	now := utc(2026, 10, 7, 12)
	if _, err := ValidateBirthdate("1899-12-31", now); !errors.Is(err, ErrBirthdateInvalid) {
		t.Errorf("1899-12-31: got %v, want ErrBirthdateInvalid", err)
	}
	if _, err := ValidateBirthdate("0001-01-01", now); !errors.Is(err, ErrBirthdateInvalid) {
		t.Errorf("0001-01-01: got %v, want ErrBirthdateInvalid", err)
	}
	if _, err := ValidateBirthdate("1900-01-01", now); err != nil {
		t.Errorf("1900-01-01 is the first allowed day: %v", err)
	}
}

func TestValidateBirthdate_RejectsFuture(t *testing.T) {
	// 05:00 UTC: plus 14 hours is still the same UTC date, so tomorrow is
	// beyond every time zone on earth.
	early := utc(2026, 10, 7, 5)
	if _, err := ValidateBirthdate("2026-10-08", early); !errors.Is(err, ErrBirthdateFuture) {
		t.Errorf("tomorrow at 05:00 UTC: got %v, want ErrBirthdateFuture", err)
	}
	if _, err := ValidateBirthdate("2026-10-07", early); err != nil {
		t.Errorf("today is not the future: %v", err)
	}
	if _, err := ValidateBirthdate("2099-01-01", early); !errors.Is(err, ErrBirthdateFuture) {
		t.Errorf("far future: got %v, want ErrBirthdateFuture", err)
	}
	// 11:00 UTC: plus 14 hours is already tomorrow (UTC+14 users), so
	// tomorrow's date is accepted here, and the day after is not.
	late := utc(2026, 10, 7, 11)
	if _, err := ValidateBirthdate("2026-10-08", late); err != nil {
		t.Errorf("tomorrow at 11:00 UTC is today in UTC+14: %v", err)
	}
	if _, err := ValidateBirthdate("2026-10-09", late); !errors.Is(err, ErrBirthdateFuture) {
		t.Errorf("day after tomorrow: got %v, want ErrBirthdateFuture", err)
	}
}

func TestValidateBirthdate_ErrorsNeverEchoInput(t *testing.T) {
	now := utc(2026, 10, 7, 5)
	for _, s := range []string{"2000-02-30", "1899-01-01", "2099-05-05", "garbage-input-xyz"} {
		_, err := ValidateBirthdate(s, now)
		if err == nil {
			t.Fatalf("%q: expected an error", s)
		}
		if strings.Contains(err.Error(), s) || strings.Contains(err.Error(), "2099") || strings.Contains(err.Error(), "1899") {
			t.Errorf("error text %q echoes the input %q", err.Error(), s)
		}
	}
}

func TestAgeOn_TurnsThirteenToday(t *testing.T) {
	now := utc(2026, 10, 7, 9)
	birth := time.Date(2013, 10, 7, 0, 0, 0, 0, time.UTC) // exactly today minus 13 years
	if got := AgeOn(birth, now); got != MinimumAge {
		t.Fatalf("AgeOn = %d, want %d", got, MinimumAge)
	}
}

func TestAgeOn_DayBeforeThirteenthIsTwelve(t *testing.T) {
	now := utc(2026, 10, 7, 9)
	birth := time.Date(2013, 10, 8, 0, 0, 0, 0, time.UTC)
	if got := AgeOn(birth, now); got != MinimumAge-1 {
		t.Fatalf("AgeOn = %d, want %d", got, MinimumAge-1)
	}
}

func TestAgeOn_LeapDayBirthdayInNonLeapYear(t *testing.T) {
	birth := time.Date(2008, 2, 29, 0, 0, 0, 0, time.UTC)
	if got := AgeOn(birth, utc(2021, 2, 28, 12)); got != 12 {
		t.Errorf("28 Feb 2021: AgeOn = %d, want 12", got)
	}
	if got := AgeOn(birth, utc(2021, 3, 1, 12)); got != 13 {
		t.Errorf("1 Mar 2021: AgeOn = %d, want 13", got)
	}
	if got := AgeOn(birth, utc(2024, 2, 29, 12)); got != 16 {
		t.Errorf("29 Feb 2024: AgeOn = %d, want 16", got)
	}
}

func TestAgeOn_UsesTupleCompareNotDayCount(t *testing.T) {
	// A day-count division (days / 365) would call this person 13 a day
	// early or late depending on leap days; the tuple compare must not.
	birth := time.Date(2013, 3, 1, 0, 0, 0, 0, time.UTC)
	if got := AgeOn(birth, utc(2026, 2, 28, 23)); got != 12 {
		t.Errorf("28 Feb 2026: AgeOn = %d, want 12", got)
	}
	if got := AgeOn(birth, utc(2026, 3, 1, 0)); got != 13 {
		t.Errorf("1 Mar 2026: AgeOn = %d, want 13", got)
	}
	birth = time.Date(2012, 12, 31, 0, 0, 0, 0, time.UTC)
	if got := AgeOn(birth, utc(2026, 1, 1, 0)); got != 13 {
		t.Errorf("1 Jan 2026: AgeOn = %d, want 13", got)
	}
}

func TestAgeOn_TomorrowBirthdayIsNegativeOrZeroAndUnderMinimum(t *testing.T) {
	now := utc(2026, 10, 7, 11)
	tomorrow := time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)
	if got := AgeOn(tomorrow, now); got >= MinimumAge {
		t.Fatalf("AgeOn(tomorrow) = %d, must be under the minimum", got)
	}
}

// The birthday date must stay unreadable: the only trace on the domain model
// is a boolean that says whether one is on file.
func TestUser_HasNoBirthdayDateField(t *testing.T) {
	typ := reflect.TypeOf(User{})
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if strings.Contains(strings.ToLower(f.Name), "birth") && !(f.Name == "HasBirthday" && f.Type.Kind() == reflect.Bool) {
			t.Errorf("User.%s (%s): only HasBirthday bool may mention a birthday", f.Name, f.Type)
		}
	}
}
