package main

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	jose "github.com/go-jose/go-jose/v4"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"pocketrelay/internal/mcpconfig"
)

func TestMCPOAuthBoundary(t *testing.T) {
	dir := t.TempDir()
	db, e := sql.Open("sqlite", filepath.Join(dir, "events.sqlite3"))
	if e != nil {
		t.Fatal(e)
	}
	_, e = db.Exec(`CREATE TABLE events(id TEXT PRIMARY KEY,payload TEXT,audio TEXT)`)
	if e != nil {
		t.Fatal(e)
	}
	id := strings.Repeat("a", 64)
	_, e = db.Exec(`INSERT INTO events VALUES(?,?,NULL)`, id, `{"type":"sms","message":"synthetic MCP test","id":"`+id+`"}`)
	if e != nil {
		t.Fatal(e)
	}
	db.Close()
	ro, e := openReadDB(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer ro.Close()
	g := &gateway{db: ro, data: dir, key: strings.Repeat("x", 64)}
	a := newMCP(g)
	c := mcpconfig.Config{Enabled: true, PublicURL: "https://phone.example", Issuer: "https://identity.example/", OwnerSubject: "owner-123"}
	private, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	signer, e := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: private}, nil)
	if e != nil {
		t.Fatal(e)
	}
	sign := func(overrides map[string]any) string {
		claims := map[string]any{"iss": c.Issuer, "sub": c.OwnerSubject, "aud": c.Resource(), "exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(), "scope": mcpconfig.Scope}
		for k, v := range overrides {
			claims[k] = v
		}
		b, _ := json.Marshal(claims)
		j, e := signer.Sign(b)
		if e != nil {
			t.Fatal(e)
		}
		s, e := j.CompactSerialize()
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	setup := func() {
		a.cached = c
		a.verifier = oidc.NewVerifier(c.Issuer, &oidc.StaticKeySet{PublicKeys: []crypto.PublicKey{&private.PublicKey}}, &oidc.Config{ClientID: c.Resource(), SupportedSigningAlgs: []string{"RS256"}})
	}
	req := func(path, token, body, origin, host string) *httptest.ResponseRecorder {
		method := "POST"
		if strings.HasPrefix(path, "/.well-known/") {
			method = "GET"
		}
		r := httptest.NewRequest(method, "https://phone.example"+path, strings.NewReader(body))
		if host != "" {
			r.Host = host
		}
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Accept", "application/json, text/event-stream")
		r.Header.Set("MCP-Protocol-Version", "2025-11-25")
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		return w
	}
	list := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	if w := req("/mcp", "", list, "", ""); w.Code != 503 {
		t.Fatalf("unconfigured %d", w.Code)
	}
	if e = mcpconfig.Save(dir, c); e != nil {
		t.Fatal(e)
	}
	setup()
	if w := req("/.well-known/oauth-protected-resource/mcp", "", "", "", ""); w.Code != 200 || !strings.Contains(w.Body.String(), c.Resource()) {
		t.Fatal("metadata failed", w.Code)
	}
	for name, token := range map[string]string{"missing": "", "legacy key": g.key, "wrong owner": sign(map[string]any{"sub": "someone-else"}), "expired": sign(map[string]any{"exp": time.Now().Add(-time.Hour).Unix()}), "wrong audience": sign(map[string]any{"aud": "different-api"}), "wrong issuer": sign(map[string]any{"iss": "https://other.example"}), "future": sign(map[string]any{"nbf": time.Now().Add(time.Hour).Unix()}), "bad signature": sign(nil) + "x"} {
		if w := req("/mcp", token, list, "", ""); w.Code != 401 || w.Header().Get("WWW-Authenticate") == "" {
			t.Fatalf("%s should reject: %d", name, w.Code)
		}
	}
	if w := req("/mcp", sign(map[string]any{"scope": "openid"}), list, "", ""); w.Code != 403 {
		t.Fatal("missing scope allowed")
	}
	good := sign(nil)
	for _, pair := range [][2]string{{"https://evil.example", ""}, {"", "evil.example"}} {
		if w := req("/mcp", good, list, pair[0], pair[1]); w.Code != 403 {
			t.Fatal("host/origin bypass")
		}
	}
	for _, body := range []string{`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`, list, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"search_messages","arguments":{"query":"synthetic"}}}`} {
		w := req("/mcp", good, body, "", "")
		if w.Code != 200 || strings.Contains(w.Body.String(), `"isError":true`) || strings.Contains(w.Body.String(), `"error":`) {
			t.Fatalf("MCP failed %d %s", w.Code, w.Body.String())
		}
		if strings.Contains(body, "search_messages") && !strings.Contains(w.Body.String(), "synthetic MCP test") {
			t.Fatal("record missing")
		}
	}
	// Real SDK client transport negotiates with the HTTP server and calls a tool.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Host = "phone.example"
		r.Header.Set("Authorization", "Bearer "+good)
		a.ServeHTTP(w, r)
	}))
	defer ts.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "integration-test", Version: "1"}, nil)
	session, e := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: ts.URL + "/mcp"}, nil)
	if e != nil {
		t.Fatal(e)
	}
	tools, e := session.ListTools(context.Background(), nil)
	if e != nil || len(tools.Tools) != 8 {
		t.Fatal("tool discovery", e)
	}
	result, e := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "search_messages", Arguments: map[string]any{"query": "synthetic"}})
	if e != nil || result.IsError {
		t.Fatal("SDK call", e)
	}
	session.Close()
	for _, args := range []string{`{"limit":501}`, `{"unknown":1}`} {
		w := req("/mcp", good, `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"search_messages","arguments":`+args+`}}`, "", "")
		if !strings.Contains(w.Body.String(), `"isError":true`) {
			t.Fatal("invalid arguments accepted")
		}
	}
	if w := req("/api/mcp-config", good, list, "", ""); w.Code != 404 {
		t.Fatal("admin exposed")
	}
	// Simulated process restart keeps configuration and read-only records; token validation rebuilt.
	a = newMCP(g)
	setup()
	if w := req("/mcp", good, list, "", ""); w.Code != 200 {
		t.Fatal("restart lost config")
	}
	// Exercise HTTPS discovery and JWKS fetch rather than an injected verifier.
	var issuer string
	idp := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("user token leaked to identity discovery")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			json.NewEncoder(w).Encode(map[string]any{"issuer": issuer, "authorization_endpoint": issuer + "authorize", "token_endpoint": issuer + "token", "jwks_uri": issuer + "jwks", "id_token_signing_alg_values_supported": []string{"RS256"}, "code_challenge_methods_supported": []string{"S256"}})
		case "/jwks":
			json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &private.PublicKey, Use: "sig", Algorithm: "RS256"}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer idp.Close()
	issuer = idp.URL + "/"
	c.Issuer = issuer
	if e = mcpconfig.Save(dir, c); e != nil {
		t.Fatal(e)
	}
	a = newMCP(g)
	a.discoveryClient = idp.Client()
	good = sign(nil)
	if w := req("/mcp", good, list, "", ""); w.Code != 200 {
		t.Fatalf("HTTPS discovery/JWKS failed: %d %s", w.Code, w.Body.String())
	}
	c.Enabled = false
	if e = mcpconfig.Save(dir, c); e != nil {
		t.Fatal(e)
	}
	if w := req("/mcp", good, list, "", ""); w.Code != 503 {
		t.Fatal("disable not enforced")
	}
}
