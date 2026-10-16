package main

import (
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestPairingLinkAndPublicIsolation(t *testing.T) {
	a := testApp(t)
	a.cfg.PublicURL = "https://phone.example"
	a.cfg.UploadKey = "test-secret-0123456789+&"
	p := pairing(a.dir, a.cfg)
	link, e := url.Parse(str(p["pairLink"]))
	if e != nil {
		t.Fatal(e)
	}
	if link.Path != "/pair" || link.RawQuery != "" || strings.Contains(link.RequestURI(), a.cfg.UploadKey) {
		t.Fatal("credentials leaked into request")
	}
	values, e := url.ParseQuery(link.EscapedFragment())
	if e != nil || values.Get("key") != a.cfg.UploadKey || values.Get("server") != a.cfg.PublicURL {
		t.Fatal("fragment roundtrip", values, e)
	}
	for _, path := range []string{"/pair", "/pair.js", "/qrcode.min.js"} {
		w := httptest.NewRecorder()
		a.ingest(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 || strings.Contains(w.Body.String(), a.cfg.UploadKey) || w.Header().Get("Referrer-Policy") != "no-referrer" {
			t.Fatal(path, w.Code)
		}
	}
	for _, path := range []string{"/api/events", "/api/connection", "/mcp"} {
		w := httptest.NewRecorder()
		a.ingest(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 404 {
			t.Fatal("private endpoint exposed", path)
		}
	}
	for _, good := range []bool{false, true} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/v1/pair/check", nil)
		if good {
			r.Header.Set("Authorization", "Bearer "+a.cfg.UploadKey)
		}
		a.ingest(w, r)
		want := 401
		if good {
			want = 200
		}
		if w.Code != want {
			t.Fatal(w.Code)
		}
		if strings.Contains(w.Body.String(), a.cfg.UploadKey) {
			t.Fatal("key leaked")
		}
	}
	a.cfg.PublicURL = ""
	if pairing(a.dir, a.cfg)["pairLink"] != "" || checkPublic(a.dir, a.cfg)["authenticated"] != false {
		t.Fatal("missing URL marked ready")
	}
}

func TestPairingJSONContainsThreeFields(t *testing.T) {
	a := testApp(t)
	a.cfg.PublicURL = "https://phone.example"
	b, e := json.Marshal(pairing(a.dir, a.cfg))
	if e != nil {
		t.Fatal(e)
	}
	for _, field := range []string{"pairLink", "phoneServer", "uploadKey"} {
		if !strings.Contains(string(b), field) {
			t.Fatal(field)
		}
	}
}
