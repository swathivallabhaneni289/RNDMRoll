package main

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strings"
	"testing"
	"time"
)

// The wiring in main() is the only place the real server starts the purge,
// hands delete account to log in, mounts Forgot password and tells sign-up
// which addresses are the admins'. Every handler test builds its own Deps, so
// a line dropped here would fail no other test. main() itself is never run
// (it reads the environment and dials DATABASE_URL), so these tests read
// main.go as Go source and check each line they name.

// accountsMainBody parses main.go and returns the body of func main.
func accountsMainBody(t *testing.T) *ast.BlockStmt {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "main.go", nil, 0)
	if err != nil {
		t.Fatalf("parse main.go: %v", err)
	}
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == "main" {
			return fn.Body
		}
	}
	t.Fatal("main.go has no func main")
	return nil
}

// accountsCallOf returns expr as a call of recv.name(...), with recv a plain
// identifier (a package or a variable), or nil.
func accountsCallOf(expr ast.Expr, recv, name string) *ast.CallExpr {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return nil
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return nil
	}
	if id, ok := sel.X.(*ast.Ident); !ok || id.Name != recv {
		return nil
	}
	return call
}

// accountsIdent reports whether expr is the identifier name.
func accountsIdent(expr ast.Expr, name string) bool {
	id, ok := expr.(*ast.Ident)
	return ok && name != "" && id.Name == name
}

// accountsAssignedFrom finds the statement of main's own body (not one inside
// a branch) that sets a variable to pkg.fn(...), and returns the variable's
// name, the call and where the statement sits ("" and nil when there is none).
func accountsAssignedFrom(body *ast.BlockStmt, pkg, fn string) (string, *ast.CallExpr, token.Pos) {
	for _, stmt := range body.List {
		as, ok := stmt.(*ast.AssignStmt)
		if !ok || len(as.Rhs) != 1 || len(as.Lhs) == 0 {
			continue
		}
		call := accountsCallOf(as.Rhs[0], pkg, fn)
		if call == nil {
			continue
		}
		if id, ok := as.Lhs[0].(*ast.Ident); ok {
			return id.Name, call, as.Pos()
		}
	}
	return "", nil, token.NoPos
}

// accountsCalls lists every call recv.name(...) anywhere in body, in source
// order. Calls inside a function literal (a goroutine, a defer) count only
// when inLiterals is set.
func accountsCalls(body *ast.BlockStmt, recv, name string, inLiterals bool) []*ast.CallExpr {
	var out []*ast.CallExpr
	ast.Inspect(body, func(n ast.Node) bool {
		if _, ok := n.(*ast.FuncLit); ok && !inLiterals {
			return false
		}
		if e, ok := n.(ast.Expr); ok {
			if call := accountsCallOf(e, recv, name); call != nil {
				out = append(out, call)
			}
		}
		return true
	})
	return out
}

// accountsDeps finds deps := httpapi.Deps{...} in main's own body and returns
// the variable's name and the literal.
func accountsDeps(t *testing.T, body *ast.BlockStmt) (string, *ast.CompositeLit) {
	t.Helper()
	for _, stmt := range body.List {
		as, ok := stmt.(*ast.AssignStmt)
		if !ok || len(as.Rhs) != 1 {
			continue
		}
		lit, ok := as.Rhs[0].(*ast.CompositeLit)
		if !ok {
			continue
		}
		if sel, ok := lit.Type.(*ast.SelectorExpr); ok && accountsIdent(sel.X, "httpapi") && sel.Sel.Name == "Deps" {
			if id, ok := as.Lhs[0].(*ast.Ident); ok {
				return id.Name, lit
			}
		}
	}
	t.Fatal("main.go: no deps := httpapi.Deps{...} in main")
	return "", nil
}

// accountsField returns the value of field in the literal, or nil.
func accountsField(lit *ast.CompositeLit, field string) ast.Expr {
	for _, elt := range lit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok && accountsIdent(kv.Key, field) {
			return kv.Value
		}
	}
	return nil
}

// accountsDepsSetTo finds where main sets deps.field to httpapi.ctor(...):
// in the literal, or by a statement of main's own body (not one inside a
// branch) before the server is built. It returns the call, or nil.
func accountsDepsSetTo(body *ast.BlockStmt, depsVar string, lit *ast.CompositeLit, field, ctor string, before token.Pos) *ast.CallExpr {
	if v := accountsField(lit, field); v != nil {
		if call := accountsCallOf(v, "httpapi", ctor); call != nil {
			return call
		}
	}
	for _, stmt := range body.List {
		as, ok := stmt.(*ast.AssignStmt)
		if !ok || len(as.Lhs) != 1 || len(as.Rhs) != 1 || as.Pos() >= before {
			continue
		}
		sel, ok := as.Lhs[0].(*ast.SelectorExpr)
		if !ok || !accountsIdent(sel.X, depsVar) || sel.Sel.Name != field {
			continue
		}
		if call := accountsCallOf(as.Rhs[0], "httpapi", ctor); call != nil {
			return call
		}
	}
	return nil
}

// accountsServerBuilt returns the name of the server variable, where
// httpapi.NewServer(deps) is called, and checks it is given deps.
func accountsServerBuilt(t *testing.T, body *ast.BlockStmt, depsVar string) (string, token.Pos) {
	t.Helper()
	serverVar, call, pos := accountsAssignedFrom(body, "httpapi", "NewServer")
	if serverVar == "" {
		t.Fatal("main.go: no server := httpapi.NewServer(...) in main")
	}
	if len(call.Args) != 1 || !accountsIdent(call.Args[0], depsVar) {
		t.Fatalf("main.go: httpapi.NewServer is not given %s", depsVar)
	}
	return serverVar, pos
}

// ACC-10: the purge that deletes accounts for good after 30 days runs only if
// main starts it. Without purger.Start every deleted account stays forever.
func TestAccounts_MainStartsThePurge(t *testing.T) {
	body := accountsMainBody(t)
	purger, call, at := accountsAssignedFrom(body, "purge", "New")
	if purger == "" {
		t.Fatal("main.go: no purger := purge.New(...) in main")
	}
	if len(call.Args) == 0 || accountsCallOf(call.Args[0], "postgres", "NewPurgeRepo") == nil {
		t.Fatal("main.go: purge.New is not given postgres.NewPurgeRepo(pool)")
	}
	started := false
	for _, stmt := range body.List {
		es, ok := stmt.(*ast.ExprStmt)
		if ok && stmt.Pos() > at && accountsCallOf(es.X, purger, "Start") != nil {
			started = true
		}
	}
	if !started {
		t.Fatalf("main.go: %s.Start(...) is not called in main after purge.New (inside a branch does not count)", purger)
	}
	// Shutdown waits for a purge in progress.
	waited := false
	ast.Inspect(body, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok && accountsIdent(sel.X, purger) && sel.Sel.Name == "Wait" {
			waited = true
		}
		return true
	})
	if !waited {
		t.Fatalf("main.go: shutdown no longer waits for %s.Wait", purger)
	}
}

// ACC-10 and ACC-07: delete account is mounted with the Postgres repository as
// its AccountDeleter (NewServer hands that to log in, so a log in inside the 30
// days cancels the deletion), and Forgot password is mounted. Both must be in
// deps before NewServer copies it.
func TestAccounts_MainWiresDeleteAccountAndForgotPassword(t *testing.T) {
	body := accountsMainBody(t)
	users, _, _ := accountsAssignedFrom(body, "postgres", "NewUserRepo")
	if users == "" {
		t.Fatal("main.go: no users := postgres.NewUserRepo(pool) in main")
	}
	depsVar, lit := accountsDeps(t, body)
	_, serverAt := accountsServerBuilt(t, body, depsVar)

	del := accountsDepsSetTo(body, depsVar, lit, "DeleteAccount", "NewDeleteAccountHandler", serverAt)
	if del == nil {
		t.Fatal("main.go: Deps.DeleteAccount is not set to httpapi.NewDeleteAccountHandler(...) before httpapi.NewServer")
	}
	if len(del.Args) != 2 || !accountsIdent(del.Args[0], users) || !accountsIdent(del.Args[1], users) {
		t.Fatalf("main.go: httpapi.NewDeleteAccountHandler must be given %s twice (the user store and its AccountDeleter)", users)
	}

	reset := accountsDepsSetTo(body, depsVar, lit, "PasswordReset", "NewPasswordResetHandler", serverAt)
	if reset == nil {
		t.Fatal("main.go: Deps.PasswordReset is not set to httpapi.NewPasswordResetHandler(...) before httpapi.NewServer")
	}
	if len(reset.Args) < 2 || accountsCallOf(reset.Args[1], "postgres", "NewPasswordResetRepo") == nil {
		t.Fatal("main.go: httpapi.NewPasswordResetHandler is not given postgres.NewPasswordResetRepo(pool) as its code store")
	}
}

// SES-05: a change in Settings emails the owner. main gives the account
// handler the sender Forgot password uses, mounts that handler, and waits for
// its emails at shutdown.
func TestAccounts_MainEmailsAPasswordChangeInSettings(t *testing.T) {
	body := accountsMainBody(t)
	depsVar, lit := accountsDeps(t, body)
	_, serverAt := accountsServerBuilt(t, body, depsVar)

	account, call, _ := accountsAssignedFrom(body, "httpapi", "NewAccountHandler")
	if account == "" {
		t.Fatal("main.go: no account handler := httpapi.NewAccountHandler(...) in main")
	}
	if !accountsIdent(accountsField(lit, "Account"), account) {
		t.Fatalf("main.go: Deps.Account is not %s", account)
	}
	reset := accountsDepsSetTo(body, depsVar, lit, "PasswordReset", "NewPasswordResetHandler", serverAt)
	if reset == nil || len(reset.Args) < 3 {
		t.Fatal("main.go: Deps.PasswordReset is not set to httpapi.NewPasswordResetHandler(...) before httpapi.NewServer")
	}
	sender, ok := reset.Args[2].(*ast.Ident)
	if !ok {
		t.Fatal("main.go: httpapi.NewPasswordResetHandler is not given its sender as a variable")
	}
	if len(call.Args) != 6 || !accountsIdent(call.Args[3], sender.Name) {
		t.Fatalf("main.go: httpapi.NewAccountHandler is not given %s, Forgot password's sender, as its sender", sender.Name)
	}
	// Shutdown waits for the emails a change started.
	waited := false
	ast.Inspect(body, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok && accountsIdent(sel.X, account) && sel.Sel.Name == "Wait" {
			waited = true
		}
		return true
	})
	if !waited {
		t.Fatalf("main.go: shutdown does not wait for %s.Wait", account)
	}
}

// ACC-02: sign-up refuses an ADMIN_EMAILS address only because main names
// them to the auth handler that is mounted, before the server is built.
func TestAccounts_MainNamesTheAdminAddressesToSignUp(t *testing.T) {
	body := accountsMainBody(t)
	cfg, _, _ := accountsAssignedFrom(body, "config", "MustLoad")
	if cfg == "" {
		t.Fatal("main.go: no cfg := config.MustLoad() in main")
	}
	authVar, _, _ := accountsAssignedFrom(body, "httpapi", "NewAuthHandler")
	if authVar == "" {
		t.Fatal("main.go: no authHandler := httpapi.NewAuthHandler(...) in main")
	}
	depsVar, lit := accountsDeps(t, body)
	if !accountsIdent(accountsField(lit, "Auth"), authVar) {
		t.Fatalf("main.go: Deps.Auth is not %s", authVar)
	}
	_, serverAt := accountsServerBuilt(t, body, depsVar)

	calls := accountsCalls(body, authVar, "SetAdminEmails", false)
	if len(calls) == 0 {
		t.Fatalf("main.go: %s.SetAdminEmails(...) is not called in main", authVar)
	}
	for _, call := range calls {
		if call.Pos() > serverAt {
			t.Fatalf("main.go: %s.SetAdminEmails is called after httpapi.NewServer", authVar)
		}
		arg, ok := call.Args[0].(*ast.SelectorExpr)
		if len(call.Args) != 1 || !ok || !accountsIdent(arg.X, cfg) || arg.Sel.Name != "AdminEmails" {
			t.Fatalf("main.go: %s.SetAdminEmails is not given %s.AdminEmails", authVar, cfg)
		}
	}
}

// The process serves the engine of the server built from deps.
func TestAccounts_MainServesThatServer(t *testing.T) {
	body := accountsMainBody(t)
	depsVar, _ := accountsDeps(t, body)
	serverVar, _ := accountsServerBuilt(t, body, depsVar)
	httpServer := ""
	ast.Inspect(body, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok || len(as.Rhs) != 1 {
			return true
		}
		rhs := as.Rhs[0]
		if u, ok := rhs.(*ast.UnaryExpr); ok && u.Op == token.AND {
			rhs = u.X
		}
		lit, ok := rhs.(*ast.CompositeLit)
		if !ok {
			return true
		}
		if sel, ok := lit.Type.(*ast.SelectorExpr); !ok || !accountsIdent(sel.X, "http") || sel.Sel.Name != "Server" {
			return true
		}
		handler := accountsField(lit, "Handler")
		if handler == nil || accountsCallOf(handler, serverVar, "Engine") == nil {
			t.Fatalf("main.go: the http.Server's Handler is not %s.Engine()", serverVar)
		}
		if id, ok := as.Lhs[0].(*ast.Ident); ok {
			httpServer = id.Name
		}
		return true
	})
	if httpServer == "" {
		t.Fatal("main.go: no http.Server is built in main")
	}
	if len(accountsCalls(body, httpServer, "ListenAndServe", true)) == 0 {
		t.Fatalf("main.go: %s.ListenAndServe() is never called", httpServer)
	}
}

// adminAddresses are the report emails' recipients: the ADMIN_EMAILS addresses
// that have an account, each once, lower case and trimmed.
func TestAccounts_AdminAddresses(t *testing.T) {
	cases := []struct {
		name    string
		all     []string
		missing []string
		want    []string
	}{
		{"lowered and trimmed, in order", []string{" Admin@Example.com", "second@example.com\t"}, nil, []string{"admin@example.com", "second@example.com"}},
		{"a repeat counts once, whatever its case", []string{"a@example.com", "A@EXAMPLE.COM", " a@example.com "}, nil, []string{"a@example.com"}},
		{"empty and blank entries are dropped", []string{"", "  ", "b@example.com"}, nil, []string{"b@example.com"}},
		{"an address with no account is left out, whatever its case", []string{"a@example.com", "b@example.com"}, []string{" B@Example.com "}, []string{"a@example.com"}},
		{"nothing left", []string{"a@example.com"}, []string{"a@example.com"}, nil},
		{"none", nil, nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := adminAddresses(tc.all, tc.missing); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("adminAddresses(%q, %q) = %q, want %q", tc.all, tc.missing, got, tc.want)
			}
		})
	}
}

// waitAll runs the shutdown waits in turn and says whether they all ended in
// time.
func TestAccounts_WaitAll(t *testing.T) {
	var order []string
	ok := waitAll(context.Background(),
		func() { order = append(order, "first") },
		func() { order = append(order, "second") },
	)
	if !ok || strings.Join(order, ",") != "first,second" {
		t.Fatalf("waitAll = %v, ran %v; want true, first then second", ok, order)
	}

	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	if waitAll(ctx, func() { <-release }) {
		t.Fatal("waitAll reported a wait that never ended as done")
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Fatalf("waitAll took %v after its context ended", took)
	}
}
