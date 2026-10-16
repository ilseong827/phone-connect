package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func pairing(dir string, c Config) map[string]any {
	server := str(links(dir, c)["phoneServer"])
	pairLink := ""
	localLink := ""
	ready := false
	if u, e := url.Parse(server); e == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && u.RawQuery == "" && u.Fragment == "" {
		fragment := url.Values{"v": {"1"}, "server": {strings.TrimRight(server, "/")}, "key": {c.UploadKey}}.Encode()
		pairLink = strings.TrimRight(server, "/") + "/pair#" + fragment
		localLink = fmt.Sprintf("http://localhost:%d/pair#%s", c.WebPort, fragment)
		ready = true
	}
	return map[string]any{"pairLink": pairLink, "phoneServer": server, "uploadKey": c.UploadKey, "localPairLink": localLink, "addressConfigured": ready, "note": "Open pairLink on PC/Dot to display QR. In the Android app select Server > Scan QR; server/key fill directly. Keep this link private. Run --check-link to verify reachability."}
}
func checkPublic(dir string, c Config) map[string]any {
	server := str(links(dir, c)["phoneServer"])
	result := map[string]any{"phoneServer": server, "reachable": false, "authenticated": false}
	p := pairing(dir, c)
	if p["addressConfigured"] != true {
		result["error"] = "No HTTPS address. Start tunnel, wait for URL, or configure --url."
		return result
	}
	client := http.Client{Timeout: 15 * time.Second, CheckRedirect: func(r *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	req, e := http.NewRequest("POST", strings.TrimRight(server, "/")+"/v1/pair/check", nil)
	if e != nil {
		result["error"] = "Invalid server URL"
		return result
	}
	req.Header.Set("Authorization", "Bearer "+c.UploadKey)
	response, e := client.Do(req)
	if e != nil {
		result["error"] = "HTTPS connection failed. Browsing Google does not prove tunnel access; check DNS, outbound tunnel ports and running daemon."
		return result
	}
	defer response.Body.Close()
	result["httpStatus"] = response.StatusCode
	result["reachable"] = true
	var body struct {
		OK      bool   `json:"ok"`
		Service string `json:"service"`
	}
	if response.StatusCode == 200 && json.NewDecoder(http.MaxBytesReader(nil, response.Body, 4096)).Decode(&body) == nil && body.OK && body.Service == "Phone Connect" {
		result["authenticated"] = true
	} else {
		result["error"] = "Server did not confirm the upload key. Check URL, key, tunnel and server version."
	}
	return result
}
func pairingAsset(w http.ResponseWriter, r *http.Request) bool {
	name := ""
	switch r.URL.Path {
	case "/pair", "/pair/":
		name = "pair.html"
	case "/pair.js":
		name = "pair.js"
	case "/qrcode.min.js":
		name = "qrcode.min.js"
	default:
		return false
	}
	if r.Method != "GET" && r.Method != "HEAD" {
		fail(w, 405, "GET required")
		return true
	}
	b, e := webFiles.ReadFile("web/" + name)
	if e != nil {
		fail(w, 404, "Not found")
		return true
	}
	ct := "text/javascript; charset=utf-8"
	if name == "pair.html" {
		ct = "text/html; charset=utf-8"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'")
	if r.Method != "HEAD" {
		w.Write(b)
	}
	return true
}
