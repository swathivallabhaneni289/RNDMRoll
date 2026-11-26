package main

import (
	"go/ast"
	"go/token"
	"testing"
)

// The sign-up code wiring in main() (developer, 2026-11-24), read as Go
// source like the accounts wiring (accounts_coverage_test.go): no other test
// runs main, so a line dropped here would fail nothing else.

// Sign-up makes an account only with the code it emailed, so without
// Deps.SignupCode nobody can sign up (sign-up answers 500). main sets it,
// before the server is built, to httpapi.NewSignupCodeHandler over the user
// store and postgres.NewPendingSignupRepo(pool), with Forgot password's
// sender and the server's secret, and shutdown waits for its emails.
func TestSignup_MainWiresTheSignUpCodes(t *testing.T) {
	body := accountsMainBody(t)
	users, _, _ := accountsAssignedFrom(body, "postgres", "NewUserRepo")
	cfg, _, _ := accountsAssignedFrom(body, "config", "MustLoad")
	if users == "" || cfg == "" {
		t.Fatal("main.go: no users := postgres.NewUserRepo(pool) or cfg := config.MustLoad() in main")
	}
	depsVar, lit := accountsDeps(t, body)
	_, serverAt := accountsServerBuilt(t, body, depsVar)

	codes := accountsDepsSetTo(body, depsVar, lit, "SignupCode", "NewSignupCodeHandler", serverAt)
	if codes == nil {
		t.Fatal("main.go: Deps.SignupCode is not set to httpapi.NewSignupCodeHandler(...) before httpapi.NewServer")
	}
	if len(codes.Args) != 4 || !accountsIdent(codes.Args[0], users) || accountsCallOf(codes.Args[1], "postgres", "NewPendingSignupRepo") == nil {
		t.Fatalf("main.go: httpapi.NewSignupCodeHandler is not given %s and postgres.NewPendingSignupRepo(pool)", users)
	}
	reset := accountsDepsSetTo(body, depsVar, lit, "PasswordReset", "NewPasswordResetHandler", serverAt)
	if reset == nil || len(reset.Args) < 3 {
		t.Fatal("main.go: Deps.PasswordReset is not set to httpapi.NewPasswordResetHandler(...) before httpapi.NewServer")
	}
	sender, ok := reset.Args[2].(*ast.Ident)
	if !ok || !accountsIdent(codes.Args[2], sender.Name) {
		t.Fatal("main.go: httpapi.NewSignupCodeHandler is not given Forgot password's sender")
	}
	if secret, ok := codes.Args[3].(*ast.SelectorExpr); !ok || !accountsIdent(secret.X, cfg) || secret.Sel.Name != "JWTSecret" {
		t.Fatalf("main.go: httpapi.NewSignupCodeHandler is not given %s.JWTSecret", cfg)
	}

	waited := false
	ast.Inspect(body, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Wait" {
			return true
		}
		if field, ok := sel.X.(*ast.SelectorExpr); ok && accountsIdent(field.X, depsVar) && field.Sel.Name == "SignupCode" {
			waited = true
		}
		return true
	})
	if !waited {
		t.Fatalf("main.go: shutdown does not wait for %s.SignupCode.Wait", depsVar)
	}
}

// The sign-ups never finished, each with a password hash, a name and a
// birthday, are deleted only if main adds their sweep to the purge loop,
// with the Postgres store, after purge.New and before purger.Start, in
// main's own body. Without it they stay forever.
func TestSignup_MainSweepsUnfinishedSignUpsInThePurgeLoop(t *testing.T) {
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
		if call := accountsExprCall(stmt, purger, "SetSignups"); call != nil {
			if len(call.Args) != 1 || accountsCallOf(call.Args[0], "postgres", "NewPendingSignupRepo") == nil {
				t.Fatalf("main.go: %s.SetSignups is not given postgres.NewPendingSignupRepo(pool)", purger)
			}
			setAt = stmt.Pos()
		}
		if accountsExprCall(stmt, purger, "Start") != nil && startAt == token.NoPos {
			startAt = stmt.Pos()
		}
	}
	if setAt == token.NoPos {
		t.Fatalf("main.go: %s.SetSignups(...) is not called in main after purge.New (inside a branch does not count)", purger)
	}
	if startAt == token.NoPos || setAt > startAt {
		t.Fatalf("main.go: %s.SetSignups must come before %s.Start", purger, purger)
	}
}
