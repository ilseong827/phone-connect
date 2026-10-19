package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"pocketrelay/internal/mcpconfig"
)

// Separate, read-only browser endpoint. Never exposes pairing keys or settings.
type webConfig struct {
	PublicURL string `json:"publicUrl"`
	ClientID  string `json:"clientId"`
}
type webFlow struct {
	state, nonce, verifier string
	expires                time.Time
}
type webSession struct {
	expires time.Time
	owner   string
}
type remoteWeb struct {
	app           *App
	cfg           webConfig
	issuer, owner string
	mu            sync.Mutex
	flows         map[string]webFlow
	sessions      map[string]webSession
	provider      *oidc.Provider
	client        *http.Client
}

func randomWebToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func newRemoteWeb(a *App) (*remoteWeb, error) {
	b, err := os.ReadFile(filepath.Join(a.dir, "web.json"))
	if err != nil {
		return nil, err
	}
	var c webConfig
	if err = json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	u, err := url.Parse(c.PublicURL)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || c.ClientID == "" {
		return nil, errors.New("invalid web.json HTTPS URL or clientId")
	}
	c.PublicURL = strings.TrimRight(c.PublicURL, "/")
	m, err := mcpconfig.Load(a.dir)
	if err != nil {
		return nil, err
	}
	if !m.Enabled {
		return nil, errors.New("MCP owner configuration is required")
	}
	return &remoteWeb{app: a, cfg: c, issuer: m.Issuer, owner: m.OwnerSubject, flows: map[string]webFlow{}, sessions: map[string]webSession{}, client: &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("identity provider redirect refused") }}}, nil
}
func (g *remoteWeb) oauth(ctx context.Context) (*oauth2.Config, *oidc.IDTokenVerifier, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	ctx = oidc.ClientContext(ctx, g.client)
	if g.provider == nil {
		p, e := oidc.NewProvider(ctx, g.issuer)
		if e != nil {
			return nil, nil, e
		}
		g.provider = p
	}
	ep := g.provider.Endpoint()
	ep.AuthStyle = oauth2.AuthStyleInParams
	for _, raw := range []string{ep.AuthURL, ep.TokenURL} {
		u, e := url.Parse(raw)
		if e != nil || u.Scheme != "https" || u.User != nil {
			return nil, nil, errors.New("insecure OAuth endpoint")
		}
	}
	c := &oauth2.Config{ClientID: g.cfg.ClientID, RedirectURL: g.cfg.PublicURL + "/auth/callback", Endpoint: ep, Scopes: []string{oidc.ScopeOpenID, "profile", "email"}}
	return c, g.provider.Verifier(&oidc.Config{ClientID: g.cfg.ClientID}), nil
}
func webCookie(w http.ResponseWriter, name, value string, age int) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/", MaxAge: age, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
}
func (g *remoteWeb) prune() {
	now := time.Now()
	for k, v := range g.flows {
		if !now.Before(v.expires) {
			delete(g.flows, k)
		}
	}
	for k, v := range g.sessions {
		if !now.Before(v.expires) {
			delete(g.sessions, k)
		}
	}
}
func (g *remoteWeb) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'unsafe-inline'; script-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
	u, _ := url.Parse(g.cfg.PublicURL)
	if r.Host != u.Host || (r.Header.Get("Origin") != "" && r.Header.Get("Origin") != g.cfg.PublicURL) {
		http.Error(w, "Forbidden", 403)
		return
	}
	// Reload owner/enable configuration to immediately invalidate access after owner changes.
	m, e := mcpconfig.Load(g.app.dir)
	if e != nil || !m.Enabled || m.OwnerSubject != g.owner || m.Issuer != g.issuer {
		http.Error(w, "Web access disabled; restart after configuration changes", 503)
		return
	}
	if r.URL.Path == "/auth/logout" {
		if r.Method != "POST" || r.Header.Get("Origin") != g.cfg.PublicURL {
			http.Error(w, "Forbidden", 403)
			return
		}
		if c, e := r.Cookie("__Host-pc-session"); e == nil {
			g.mu.Lock()
			delete(g.sessions, digest([]byte(c.Value)))
			g.mu.Unlock()
		}
		webCookie(w, "__Host-pc-session", "", -1)
		http.Redirect(w, r, "/signed-out", http.StatusSeeOther)
		return
	}
	if r.Method != "GET" && r.Method != "HEAD" {
		http.Error(w, "Read only", 405)
		return
	}
	switch r.URL.Path {
	case "/auth/login":
		g.login(w, r)
		return
	case "/auth/callback":
		g.callback(w, r)
		return
	case "/signed-out":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><meta name="viewport" content="width=device-width,initial-scale=1"><title>Phone Connect</title><h1>로그아웃했습니다</h1><a href="/auth/login">Google로 다시 로그인</a>`)
		return
	}
	authenticated := false
	if c, e := r.Cookie("__Host-pc-session"); e == nil {
		g.mu.Lock()
		s, ok := g.sessions[digest([]byte(c.Value))]
		authenticated = ok && s.owner == g.owner && time.Now().Before(s.expires)
		g.mu.Unlock()
	}
	if !authenticated {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/auth/login", 302)
		} else {
			http.Error(w, "Authentication required", 401)
		}
		return
	}
	p := r.URL.Path
	if p == "/remote.js" {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		fmt.Fprint(w, remoteJS)
		return
	}
	if p == "/" {
		b, _ := webFiles.ReadFile("web/chat.html")
		s := strings.Replace(string(b), `<script src="/chat.js"`, `<script src="/remote.js" defer></script><script src="/chat.js"`, 1)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, s)
		return
	}
	allowed := p == "/chat.js" || p == "/qrcode.min.js" || p == "/api/events"
	for _, prefix := range []string{"/api/record/", "/audio/", "/image/", "/file/"} {
		if strings.HasPrefix(p, prefix) && validID.MatchString(strings.TrimPrefix(p, prefix)) {
			allowed = true
		}
	}
	if !allowed {
		http.NotFound(w, r)
		return
	}
	g.app.viewer(w, r)
}
func (g *remoteWeb) login(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	c, _, e := g.oauth(ctx)
	if e != nil {
		http.Error(w, "Login provider unavailable", 503)
		return
	}
	f := webFlow{state: randomWebToken(), nonce: randomWebToken(), verifier: oauth2.GenerateVerifier(), expires: time.Now().Add(5 * time.Minute)}
	key := randomWebToken()
	g.mu.Lock()
	g.prune()
	if len(g.flows) >= 4096 {
		g.mu.Unlock()
		http.Error(w, "Try again later", 429)
		return
	}
	g.flows[digest([]byte(key))] = f
	g.mu.Unlock()
	webCookie(w, "__Host-pc-flow", key, 300)
	http.Redirect(w, r, c.AuthCodeURL(f.state, oauth2.S256ChallengeOption(f.verifier), oidc.Nonce(f.nonce), oauth2.SetAuthURLParam("connection", "google-oauth2")), 302)
}
func (g *remoteWeb) callback(w http.ResponseWriter, r *http.Request) {
	cookie, e := r.Cookie("__Host-pc-flow")
	if e != nil {
		http.Error(w, "Login expired; start again", 400)
		return
	}
	g.mu.Lock()
	f, ok := g.flows[digest([]byte(cookie.Value))]
	delete(g.flows, digest([]byte(cookie.Value)))
	g.mu.Unlock()
	webCookie(w, "__Host-pc-flow", "", -1)
	if !ok || !time.Now().Before(f.expires) || r.URL.Query().Get("state") != f.state || r.URL.Query().Get("code") == "" {
		http.Error(w, "Invalid login response", 400)
		return
	}
	ctx, cancel := context.WithTimeout(oidc.ClientContext(r.Context(), g.client), 15*time.Second)
	defer cancel()
	c, v, e := g.oauth(ctx)
	if e != nil {
		http.Error(w, "Login unavailable", 503)
		return
	}
	t, e := c.Exchange(ctx, r.URL.Query().Get("code"), oauth2.VerifierOption(f.verifier))
	if e != nil {
		http.Error(w, "Login exchange failed", 401)
		return
	}
	raw, _ := t.Extra("id_token").(string)
	id, e := v.Verify(ctx, raw)
	if e != nil || id.Subject != g.owner || id.Nonce != f.nonce || !time.Now().Before(id.Expiry) {
		http.Error(w, "This Google account is not the server owner", 403)
		return
	}
	var claims struct {
		AuthorizedParty string `json:"azp"`
	}
	if id.Claims(&claims) != nil || (claims.AuthorizedParty != "" && claims.AuthorizedParty != g.cfg.ClientID) || (len(id.Audience) > 1 && claims.AuthorizedParty != g.cfg.ClientID) {
		http.Error(w, "Invalid login audience", 403)
		return
	}
	expires := time.Now().Add(8 * time.Hour)
	if id.Expiry.Before(expires) {
		expires = id.Expiry
	}
	key := randomWebToken()
	g.mu.Lock()
	g.prune()
	if len(g.sessions) >= 4096 {
		g.mu.Unlock()
		http.Error(w, "Too many sessions", 429)
		return
	}
	g.sessions[digest([]byte(key))] = webSession{expires: expires, owner: id.Subject}
	g.mu.Unlock()
	webCookie(w, "__Host-pc-session", key, int(time.Until(expires).Seconds()))
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

const remoteJS = `'use strict';
document.getElementById('connect').hidden=true;
const form=document.createElement('form');form.method='post';form.action='/auth/logout';
const button=document.createElement('button');button.textContent='로그아웃';form.append(button);document.querySelector('.rail').append(form);
const originalFetch=window.fetch.bind(window);window.fetch=async(...args)=>{const r=await originalFetch(...args);if(r.status===401){location.assign('/auth/login');throw Error('Login expired');}return r;};
`
