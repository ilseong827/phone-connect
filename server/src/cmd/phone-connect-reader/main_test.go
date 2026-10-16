package main

import (
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadBoundaryAndQueries(t *testing.T) {
	dir := t.TempDir()
	db, e := sql.Open("sqlite", filepath.Join(dir, "events.sqlite3"))
	if e != nil {
		t.Fatal(e)
	}
	_, e = db.Exec("CREATE TABLE events(id TEXT PRIMARY KEY,payload TEXT,audio TEXT)")
	if e != nil {
		t.Fatal(e)
	}
	ids := []string{strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("c", 64), strings.Repeat("d", 64)}
	values := []map[string]any{
		{"id": ids[0], "type": "contacts", "deviceId": "phone", "entries": []map[string]string{{"address": "0101234", "displayName": "홍길동"}}},
		{"id": ids[1], "type": "sms", "deviceId": "phone", "address": "0101234", "message": "일정 확인", "direction": "sent"},
		{"id": ids[2], "type": "kakao", "chatRoom": "테스트방", "sender": "친구", "message": "테스트"},
		{"id": ids[3], "type": "recording", "fileName": "test.m4a"},
	}
	for _, v := range values {
		b, _ := json.Marshal(v)
		if _, e = db.Exec("INSERT INTO events VALUES(?,?,NULL)", v["id"], string(b)); e != nil {
			t.Fatal(e)
		}
	}
	db.Close()
	ro, e := openReadDB(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer ro.Close()
	g := &gateway{db: ro, key: strings.Repeat("x", 64), data: dir}
	if _, e = ro.Exec("DELETE FROM events"); e == nil {
		t.Fatal("database writable")
	}
	request := func(method, path, key, body string) int {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+key)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		g.ServeHTTP(w, r)
		return w.Code
	}
	for _, key := range []string{"", "upload-key", "admin-key"} {
		if request("POST", "/v1/read/relay_status", key, "{}") != 401 {
			t.Fatal("wrong key accepted")
		}
	}
	for _, path := range []string{"/api/connection", "/api/control/stop", "/v1/events", "/pair", "/mcp"} {
		if request("POST", path, g.key, "{}") != 404 {
			t.Fatal("private route exposed", path)
		}
	}
	if request("POST", "/v1/read/search_messages", g.key, `{"limit":501}`) != 400 {
		t.Fatal("unbounded query")
	}
	if request("POST", "/v1/read/search_messages", g.key, `{"unknown":1}`) != 400 {
		t.Fatal("unknown argument")
	}
	if request("POST", "/v1/read/relay_status", g.key, `{} {}`) != 400 {
		t.Fatal("trailing json")
	}
	if request("POST", "/v1/read/relay_status", g.key, `{}`) != 200 {
		t.Fatal("status failed")
	}
	result, e := g.call("search_messages", arguments{Query: "홍길동"})
	if e != nil {
		t.Fatal(e)
	}
	page := result.(map[string]any)
	events := page["events"].([]map[string]any)
	if len(events) != 1 || events[0]["id"] != ids[1] || events[0]["direction"] != "sent" {
		t.Fatal("contact resolution/search failed")
	}
	first, _ := g.call("search_messages", arguments{Limit: 1})
	p := first.(map[string]any)
	if p["more"] != true {
		t.Fatal("pagination missing")
	}
	second, _ := g.call("search_messages", arguments{After: p["cursor"].(int64), Limit: 1})
	if second.(map[string]any)["events"].([]map[string]any)[0]["id"] != ids[2] {
		t.Fatal("pagination skipped")
	}
	if _, e = g.record(ids[0]); e == nil {
		t.Fatal("bulk contacts exposed")
	}
	v, e := g.record(ids[3])
	if e != nil || v["transcript"].(map[string]any)["status"] != "not_transcribed" {
		t.Fatal("missing transcription misreported")
	}
	os.WriteFile(filepath.Join(dir, ids[3]+".audio"), []byte("audio-test"), 0600)
	if request("GET", "/v1/audio/"+ids[3], g.key, "") != 200 {
		t.Fatal("audio unavailable")
	}
	if request("GET", "/v1/audio/"+ids[1], g.key, "") != 404 {
		t.Fatal("non-recording download")
	}
	r := httptest.NewRequest("GET", "/v1/tools", nil)
	r.Header.Set("Authorization", "Bearer "+g.key)
	r.Header.Set("Origin", "https://other.example")
	w := httptest.NewRecorder()
	g.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("browser origin accepted")
	}
}
