package postgres

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// CheckTestDatabaseURL refuses any database URL whose database name does not
// end in "_test". Test suites in this repo truncate tables and run migrations
// against TEST_DATABASE_URL, so a URL that points at a real database (for
// example rndmroll_dev) must stop the test process before anything runs.
//
// It is exported only because two test packages (this one and httpapi) call
// it from TestMain; production code never does.
//
// Only the URL form is accepted: the key=value form is rejected, as is a
// dbname query parameter (libpq and pgx let it override the path), so the
// name checked here is the name the connection uses. The error text never
// includes the URL, which can hold a password.
func CheckTestDatabaseURL(raw string) error {
	if !strings.HasPrefix(raw, "postgres://") && !strings.HasPrefix(raw, "postgresql://") {
		return errors.New("test database URL must start with postgres:// or postgresql:// (the key=value form is not accepted)")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return errors.New("test database URL does not parse")
	}
	if u.Query().Has("dbname") {
		return errors.New("test database URL must not set a dbname query parameter")
	}
	name := strings.TrimPrefix(u.Path, "/")
	if !strings.HasSuffix(name, "_test") {
		return fmt.Errorf("refusing to run tests: database name %q does not end in _test", name)
	}
	return nil
}
