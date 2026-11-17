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

// The bell's address for a second look is CONTACT_EMAIL and nothing else
// only because main hands cfg.ContactEmail to the bell's service, the one
// the bell's handler is built on, before the server is built.
func TestModeration_MainGivesTheBellTheContactAddress(t *testing.T) {
	body := accountsMainBody(t)
	cfg, _, _ := accountsAssignedFrom(body, "config", "MustLoad")
	if cfg == "" {
		t.Fatal("main.go: no cfg := config.MustLoad() in main")
	}
	svc, _, at := accountsAssignedFrom(body, "notifications", "NewService")
	if svc == "" {
		t.Fatal("main.go: no notificationsService := notifications.NewService(...) in main")
	}
	depsVar, lit := accountsDeps(t, body)
	_, serverAt := accountsServerBuilt(t, body, depsVar)
	handler := accountsDepsSetTo(body, depsVar, lit, "Notifications", "NewNotificationsHandler", serverAt)
	if handler == nil || len(handler.Args) == 0 || !accountsIdent(handler.Args[0], svc) {
		t.Fatalf("main.go: Deps.Notifications is not httpapi.NewNotificationsHandler(%s, ...)", svc)
	}
	set := false
	for _, stmt := range body.List {
		call := accountsExprCall(stmt, svc, "SetContact")
		if call == nil || stmt.Pos() < at || stmt.Pos() > serverAt || len(call.Args) != 1 {
			continue
		}
		if arg, ok := call.Args[0].(*ast.SelectorExpr); ok && accountsIdent(arg.X, cfg) && arg.Sel.Name == "ContactEmail" {
			set = true
		}
	}
	if !set {
		t.Fatalf("main.go: %s.SetContact(%s.ContactEmail) is not called in main between notifications.NewService and httpapi.NewServer", svc, cfg)
	}
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
