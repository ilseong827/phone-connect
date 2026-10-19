package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	jose "github.com/go-jose/go-jose/v4"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"pocketrelay/internal/mcpconfig"
	"strings"
	"testing"
	"time"
)

func TestRemoteWebBoundaries(t *testing.T) {
	dir := t.TempDir()
	cfg := mcpconfig.Config{Enabled: true, PublicURL: "https://mcp.example.com", Issuer: "https://login.example.com/", OwnerSubject: "google-oauth2|owner"}
	if e := mcpconfig.Save(dir, cfg); e != nil {
		t.Fatal(e)
	}
	os.WriteFile(filepath.Join(dir, "web.json"), []byte(`{"publicUrl":"https://view.example.com","clientId":"web-client"}`), 0600)
	g, e := newRemoteWeb(&App{dir: dir})
	if e != nil {
		t.Fatal(e)
	}
	g.sessions[digest([]byte("test-session"))] = webSession{owner: g.owner, expires: time.Now().Add(time.Hour)}
	for _, tt := range []struct {
		method, path, origin, host, cookie string
		want                               int
	}{
		{"GET", "/api/events", "", "", "", 401},
		{"GET", "/audio/" + strings.Repeat("a", 64), "", "", "", 401},
		{"GET", "/", "", "", "", 302},
		{"GET", "/", "", "evil.example", "test-session", 403},
		{"GET", "/", "https://evil.example", "", "test-session", 403},
		{"GET", "/api/connection", "", "", "test-session", 404},
		{"GET", "/api/dot-connection", "", "", "test-session", 404},
		{"GET", "/api/mcp-config", "", "", "test-session", 404},
		{"GET", "/api/control/state", "", "", "test-session", 404},
		{"POST", "/api/mcp-config", "https://view.example.com", "", "test-session", 405},
		{"GET", "/", "", "", "test-session", 200},
		{"GET", "/auth/callback?code=x&state=x", "", "", "", 400},
		{"POST", "/auth/logout", "https://evil.example", "", "test-session", 403},
	} {
		r := httptest.NewRequest(tt.method, "https://view.example.com"+tt.path, nil)
		if tt.origin != "" {
			r.Header.Set("Origin", tt.origin)
		}
		if tt.host != "" {
			r.Host = tt.host
		}
		if tt.cookie != "" {
			r.AddCookie(&http.Cookie{Name: "__Host-pc-session", Value: tt.cookie})
		}
		w := httptest.NewRecorder()
		g.ServeHTTP(w, r)
		if w.Code != tt.want {
			t.Errorf("%s %s: %d want %d", tt.method, tt.path, w.Code, tt.want)
		}
	}
	r := httptest.NewRequest("POST", "https://view.example.com/auth/logout", nil)
	r.Header.Set("Origin", "https://view.example.com")
	r.AddCookie(&http.Cookie{Name: "__Host-pc-session", Value: "test-session"})
	w := httptest.NewRecorder()
	g.ServeHTTP(w, r)
	if w.Code != 303 || len(g.sessions) != 0 {
		t.Fatal("logout did not revoke session")
	}
	cfg.Enabled = false
	mcpconfig.Save(dir, cfg)
	w = httptest.NewRecorder()
	g.ServeHTTP(w, httptest.NewRequest("GET", "https://view.example.com/", nil))
	if w.Code != 503 {
		t.Fatal("disabled config allowed access")
	}
}

func TestRemoteWebOAuthFlow(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	signer, _ := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, nil)
	var issuer, nonce, challenge, variant string
	var exchanges int
	idp := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			json.NewEncoder(w).Encode(map[string]any{"issuer": issuer, "authorization_endpoint": issuer + "/authorize", "token_endpoint": issuer + "/token", "jwks_uri": issuer + "/keys", "id_token_signing_alg_values_supported": []string{"RS256"}})
		case "/keys":
			json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, Algorithm: "RS256", Use: "sig"}}})
		case "/token":
			exchanges++
			r.ParseForm()
			sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
			if base64.RawURLEncoding.EncodeToString(sum[:]) != challenge || r.Form.Get("redirect_uri") != "https://view.example.com/auth/callback" || r.Form.Get("client_id") != "client" {
				http.Error(w, "bad PKCE", 400)
				return
			}
			claims := map[string]any{"iss": issuer, "sub": "owner", "aud": "client", "nonce": nonce, "iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix()}
			switch variant {
			case "owner":
				claims["sub"] = "other"
			case "nonce":
				claims["nonce"] = "other"
			case "audience":
				claims["aud"] = "other"
			case "expired":
				claims["exp"] = time.Now().Add(-time.Hour).Unix()
			}
			b, _ := json.Marshal(claims)
			signed, _ := signer.Sign(b)
			jwt, _ := signed.CompactSerialize()
			json.NewEncoder(w).Encode(map[string]any{"access_token": "unused", "token_type": "Bearer", "id_token": jwt})
		default:
			http.NotFound(w, r)
		}
	}))
	defer idp.Close()
	issuer = idp.URL
	dir := t.TempDir()
	mcpconfig.Save(dir, mcpconfig.Config{Enabled: true, PublicURL: "https://mcp.example.com", Issuer: issuer, OwnerSubject: "owner"})
	os.WriteFile(filepath.Join(dir, "web.json"), []byte(`{"publicUrl":"https://view.example.com","clientId":"client"}`), 0600)
	g, e := newRemoteWeb(&App{dir: dir})
	if e != nil {
		t.Fatal(e)
	}
	g.client = idp.Client()
	for _, v := range []string{"owner", "nonce", "audience", "expired", "state", "success"} {
		variant = v
		w := httptest.NewRecorder()
		g.ServeHTTP(w, httptest.NewRequest("GET", "https://view.example.com/auth/login", nil))
		if w.Code != 302 {
			t.Fatal(w.Code, w.Body.String())
		}
		redirect, _ := url.Parse(w.Header().Get("Location"))
		nonce = redirect.Query().Get("nonce")
		challenge = redirect.Query().Get("code_challenge")
		state := redirect.Query().Get("state")
		if redirect.Query().Get("code_challenge_method") != "S256" || nonce == "" || state == "" {
			t.Fatal("missing login protection")
		}
		flow := w.Result().Cookies()[0]
		if !flow.Secure || !flow.HttpOnly || flow.SameSite != http.SameSiteLaxMode {
			t.Fatal("unsafe cookie")
		}
		if v == "state" {
			state = "wrong"
		}
		r := httptest.NewRequest("GET", "https://view.example.com/auth/callback?code=test&state="+url.QueryEscape(state), nil)
		r.AddCookie(flow)
		before := exchanges
		w = httptest.NewRecorder()
		g.ServeHTTP(w, r)
		want := 403
		if v == "state" {
			want = 400
			if exchanges != before {
				t.Fatal("exchanged with wrong state")
			}
		}
		if v == "success" {
			want = 303
		}
		if w.Code != want {
			t.Fatalf("%s: %d %s", v, w.Code, w.Body.String())
		}
		if v == "success" {
			found := false
			for _, c := range w.Result().Cookies() {
				if c.Name == "__Host-pc-session" {
					found = true
					if !c.Secure || !c.HttpOnly {
						t.Fatal("unsafe session")
					}
				}
			}
			if !found {
				t.Fatal("no session")
			}
		}
		replay := httptest.NewRecorder()
		g.ServeHTTP(replay, r)
		if replay.Code != 400 {
			t.Fatal("flow replay accepted")
		}
	}
}
