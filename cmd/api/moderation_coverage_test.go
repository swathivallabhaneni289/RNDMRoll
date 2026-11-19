package main

import (
	"go/ast"
	"go/token"
	"testing"
)

// The moderation wiring in main(), read as Go source like the accounts
// wiring (accounts_coverage_test.go): no other test runs main, so a line
// dropped here would fail nothing else.

// accountsExprCall returns stmt's call of recv.name(...) when stmt is a
// plain call statement of main's own body, or nil.
func accountsExprCall(stmt ast.Stmt, recv, name string) *ast.CallExpr {
	es, ok := stmt.(*ast.ExprStmt)
	if !ok {
		return nil
	}
	return accountsCallOf(es.X, recv, name)
}

// The twin of TestAccounts_MainStartsThePurge: the posts and comments an
// admin removed are deleted for good after the 30 days only if main adds
// them to the purge loop, purger.SetRemoved with the Postgres store, in
// main's own body after purge.New and before purger.Start (the loop reads
// it, so it is set before the loop begins). Without it they stay forever,
// hidden from everyone. The loop and the shutdown wait are the account
// purge's own.
func TestModeration_MainPurgesWhatWasRemovedInThePurgeLoop(t *testing.T) {
	body := accountsMainBody(t)
	purger, _, at := accountsAssignedFrom(body, "purge", "New")
	if purger == "" {
		t.Fatal("main.go: no purger := purge.New(...) in main")
	}
	setAt, startAt := token.NoPos, token.NoPos
	for _, stmt := range body.List {
		if stmt.Pos() < at {
			continue
		}
		if call := accountsExprCall(stmt, purger, "SetRemoved"); call != nil {
			if len(call.Args) != 1 || accountsCallOf(call.Args[0], "postgres", "NewRemovedPurgeRepo") == nil {
				t.Fatalf("main.go: %s.SetRemoved is not given postgres.NewRemovedPurgeRepo(pool)", purger)
			}
			setAt = stmt.Pos()
		}
		if accountsExprCall(stmt, purger, "Start") != nil && startAt == token.NoPos {
			startAt = stmt.Pos()
		}
	}
	if setAt == token.NoPos {
		t.Fatalf("main.go: %s.SetRemoved(...) is not called in main after purge.New (inside a branch does not count)", purger)
	}
	if startAt == token.NoPos || setAt > startAt {
		t.Fatalf("main.go: %s.SetRemoved must come before %s.Start", purger, purger)
	}
}

// Second looks in the app (developer, 2026-11-19) are served only because
// main builds the service over the Postgres store, mounts its handler before
// the server is built, and hands it the admin email that the reports use, so
// each new request is emailed like an urgent report.
func TestModeration_MainWiresSecondLooksAndTheirEmail(t *testing.T) {
	body := accountsMainBody(t)
	svc, ctor, _ := accountsAssignedFrom(body, "secondlook", "NewService")
	if svc == "" {
		t.Fatal("main.go: no := secondlook.NewService(...) in main")
	}
	if len(ctor.Args) != 2 || accountsCallOf(ctor.Args[0], "postgres", "NewSecondLookRepo") == nil {
		t.Fatal("main.go: secondlook.NewService is not given postgres.NewSecondLookRepo(pool)")
	}
	depsVar, lit := accountsDeps(t, body)
	_, serverAt := accountsServerBuilt(t, body, depsVar)
	h := accountsDepsSetTo(body, depsVar, lit, "SecondLooks", "NewSecondLookHandler", serverAt)
	if h == nil || len(h.Args) != 1 || !accountsIdent(h.Args[0], svc) {
		t.Fatalf("main.go: Deps.SecondLooks is not set to httpapi.NewSecondLookHandler(%s) before httpapi.NewServer", svc)
	}
	board, _, _ := accountsAssignedFrom(body, "board", "NewService")
	reports := accountsCalls(body, board, "SetReportNotifier", false)
	looks := accountsCalls(body, svc, "SetNotifier", false)
	if len(reports) != 1 || len(looks) != 1 || len(looks[0].Args) != 1 {
		t.Fatalf("main.go: want one %s.SetReportNotifier and one %s.SetNotifier, found %d and %d", board, svc, len(reports), len(looks))
	}
	alerts, ok := reports[0].Args[0].(*ast.Ident)
	if !ok || !accountsIdent(looks[0].Args[0], alerts.Name) {
		t.Fatalf("main.go: %s.SetNotifier is not given the report email's alerts", svc)
	}
}

// The photos an owner adds to a second look are deleted 30 days after the
// removal only if main adds the sweep to the purge loop, with the Postgres
// store, after purge.New and before purger.Start, in main's own body.
// Without it they stay in the owner's folder until the account goes.
func TestModeration_MainSweepsSecondLookPhotosInThePurgeLoop(t *testing.T) {
	body := accountsMainBody(t)
	purger, _, at := accountsAssignedFrom(body, "purge", "New")
	if purger == "" {
		t.Fatal("main.go: no purger := purge.New(...) in main")
	}
	setAt, startAt := token.NoPos, token.NoPos
	for _, stmt := range body.List {
		if stmt.Pos() < at {
			continue
		}
		if call := accountsExprCall(stmt, purger, "SetSecondLooks"); call != nil {
			if len(call.Args) != 1 || accountsCallOf(call.Args[0], "postgres", "NewSecondLookPurgeRepo") == nil {
				t.Fatalf("main.go: %s.SetSecondLooks is not given postgres.NewSecondLookPurgeRepo(pool)", purger)
			}
			setAt = stmt.Pos()
		}
		if accountsExprCall(stmt, purger, "Start") != nil && startAt == token.NoPos {
			startAt = stmt.Pos()
		}
	}
	if setAt == token.NoPos {
		t.Fatalf("main.go: %s.SetSecondLooks(...) is not called in main after purge.New (inside a branch does not count)", purger)
	}
	if startAt == token.NoPos || setAt > startAt {
		t.Fatalf("main.go: %s.SetSecondLooks must come before %s.Start", purger, purger)
	}
}
