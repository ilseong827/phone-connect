package main

import (
	"net/http"
	"net/http/httptest"
	"pocketrelay/internal/mcpconfig"
	"strings"
	"testing"
)

func TestMCPSettingsLocalOnly(t *testing.T) {
	dir := t.TempDir()
	a := &App{dir: dir}
	h := private(8788, http.HandlerFunc(a.viewer))
	body := `{"enabled":true,"publicUrl":"https://phone.example","issuer":"https://identity.example/","ownerSubject":"owner"}`
	request := func(host, origin, header, ct string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "http://"+host+"/api/mcp-config", strings.NewReader(body))
		r.Header.Set("Origin", origin)
		r.Header.Set("X-Phone-Connect", header)
		r.Header.Set("Content-Type", ct)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	for _, v := range [][4]string{{"evil.example", "", "local-settings", "application/json"}, {"localhost:8788", "https://evil.example", "local-settings", "application/json"}, {"localhost:8788", "", "", "application/json"}, {"localhost:8788", "", "local-settings", "text/plain"}} {
		if w := request(v[0], v[1], v[2], v[3]); w.Code != 403 {
			t.Fatalf("unsafe settings accepted %d", w.Code)
		}
	}
	if w := request("localhost:8788", "http://localhost:8788", "local-settings", "application/json"); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	c, e := mcpconfig.Load(dir)
	if e != nil || c.OwnerSubject != "owner" {
		t.Fatal("config persistence", e)
	}
}
