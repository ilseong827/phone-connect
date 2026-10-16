package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDaemonLifecycle(t *testing.T) {
	exe := os.Getenv("POCKETRELAY_TEST_BINARY")
	if exe == "" {
		t.Skip("set POCKETRELAY_TEST_BINARY for executable lifecycle test")
	}
	dir := t.TempDir()
	c, e := config(dir, true)
	if e != nil {
		t.Fatal(e)
	}
	ports := []int{}
	for i := 0; i < 3; i++ {
		l, e := net.Listen("tcp", "127.0.0.1:0")
		if e != nil {
			t.Fatal(e)
		}
		ports = append(ports, l.Addr().(*net.TCPAddr).Port)
		l.Close()
	}
	c.WebPort = ports[0]
	c.IngestPort = ports[1]
	c.MCPPort = ports[2]
	saveConfig(dir, c)
	run := func(args ...string) []byte {
		t.Helper()
		args = append(args, "--data", dir)
		b, e := exec.Command(exe, args...).CombinedOutput()
		if e != nil {
			t.Fatalf("%v: %v %s", args, e, b)
		}
		return b
	}
	run("start")
	defer func() { exec.Command(exe, "stop", "--data", dir).Run() }()
	if s, e := admin(c, "state", "GET"); e != nil || s["running"] != true {
		t.Fatal(s, e)
	}
	v := event("sms", "real-http")
	body, _ := json.Marshal(v)
	if e := postJSON(fmt.Sprintf("http://127.0.0.1:%d/v1/events", c.IngestPort), c.UploadKey, v); e == nil {
		t.Fatal("missing idempotency key accepted")
	}
	req, _ := http.NewRequest("POST", fmt.Sprintf("http://127.0.0.1:%d/v1/events", c.IngestPort), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+c.UploadKey)
	req.Header.Set("Idempotency-Key", str(v["id"]))
	res, e := http.DefaultClient.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatal(res.StatusCode)
	}
	if !bytes.Contains(run("query", "--type", "sms"), []byte("real-http")) {
		if !bytes.Contains(run("query", "--type", "sms"), []byte(str(v["id"]))) {
			t.Fatal("record missing")
		}
	}
	run("restart")
	run("--sate")
	run("stop")
	if _, e := admin(c, "state", "GET"); e == nil {
		t.Fatal("still running")
	}
}

func TestRealSpeech(t *testing.T) {
	root := os.Getenv("POCKETRELAY_SPEECH_TEST")
	if root == "" {
		t.Skip("set POCKETRELAY_SPEECH_TEST to whisper.cpp fixture directory")
	}
	a := testApp(t)
	a.cfg.FFmpeg = "ffmpeg"
	a.cfg.Whisper = filepath.Join(root, "build/bin/whisper-cli")
	a.cfg.Model = filepath.Join(root, "models/ggml-tiny.bin")
	a.cfg.Language = "en"
	id := digest([]byte("public-jfk-fixture"))
	source := filepath.Join(root, "samples/jfk.wav")
	dest := filepath.Join(a.dir, id+".audio")
	cmd := exec.Command("ffmpeg", "-nostdin", "-v", "error", "-y", "-i", source, "-c:a", "aac", "-f", "mp4", dest)
	if b, e := cmd.CombinedOutput(); e != nil {
		t.Fatal(e, string(b))
	}
	wav, e := a.wav(context.Background(), id)
	if e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(wav)
	if e != nil || len(b) < 44 || string(b[:4]) != "RIFF" {
		t.Fatal("invalid playback WAV", e)
	}
	text, e := a.transcribe(context.Background(), id)
	if e != nil || !strings.Contains(strings.ToLower(text), "country") {
		t.Fatal("real transcription", e, text)
	}
	t.Log("AAC recording converted to playable WAV and transcribed with native whisper.cpp")
}

func testApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	c, e := config(dir, true)
	if e != nil {
		t.Fatal(e)
	}
	db, e := openDB(dir)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	return &App{dir: dir, cfg: c, db: db, stop: make(chan struct{})}
}
func event(kind, seed string) map[string]any {
	return map[string]any{"schemaVersion": float64(1), "id": digest([]byte(seed)), "type": kind, "message": "hello", "deviceId": "phone1", "messageTime": float64(1700000000000)}
}
func upload(a *App, path, id string, b []byte, ct string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", path, bytes.NewReader(b))
	r.Header.Set("Authorization", "Bearer "+a.cfg.UploadKey)
	r.Header.Set("Idempotency-Key", id)
	r.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	a.ingest(w, r)
	return w
}
func sendEvent(t *testing.T, a *App, v map[string]any) {
	t.Helper()
	b, _ := json.Marshal(v)
	w := upload(a, "/v1/events", str(v["id"]), b, "application/json")
	if w.Code != 200 {
		t.Fatalf("event: %d %s", w.Code, w.Body.String())
	}
}
func TestIngestionContactsPagination(t *testing.T) {
	a := testApp(t)
	c := event("contacts", "contacts")
	c["entries"] = []any{map[string]any{"address": "01012345678", "displayName": "테스트 친구"}}
	sendEvent(t, a, c)
	for i := 0; i < 3; i++ {
		v := event("sms", fmt.Sprint(i))
		v["address"] = "01012345678"
		v["direction"] = "outgoing"
		sendEvent(t, a, v)
		sendEvent(t, a, v)
	}
	page, e := a.query("sms", "테스트 친구", 0, 2)
	if e != nil {
		t.Fatal(e)
	}
	if len(page["events"].([]map[string]any)) != 2 || !page["more"].(bool) {
		t.Fatal(page)
	}
	next, _ := a.query("sms", "", page["cursor"].(int64), 2)
	if len(next["events"].([]map[string]any)) != 1 || next["more"].(bool) {
		t.Fatal(next)
	}
	w := httptest.NewRecorder()
	a.ingest(w, httptest.NewRequest("POST", "/v1/events", strings.NewReader("{}")))
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
}
func TestAttachmentsAndRange(t *testing.T) {
	a := testApp(t)
	data := []byte("GIF89a0123456789")
	id := digest(data)
	if w := upload(a, "/v1/attachments/"+id, id, data, "image/"); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if w := upload(a, "/v1/attachments/"+id, id, []byte("bad"), "image/"); w.Code != 400 {
		t.Fatal(w.Code)
	}
	v := event("file", "file")
	v["attachments"] = []any{map[string]any{"fileId": id, "fileName": "사진.gif", "size": float64(len(data)), "mimeType": "image/"}}
	sendEvent(t, a, v)
	got, _ := a.record(str(v["id"]))
	att := got["attachments"].([]any)[0].(map[string]any)
	if att["mimeType"] != "image/gif" {
		t.Fatal(att)
	}
	r := httptest.NewRequest("GET", "/file/"+id+"?preview=1", nil)
	r.Header.Set("Range", "bytes=0-5")
	w := httptest.NewRecorder()
	a.media(w, r)
	if w.Code != 206 || w.Body.String() != "GIF89a" {
		t.Fatal(w.Code, w.Body.String())
	}
	r = httptest.NewRequest("GET", "/file/"+id, nil)
	r.Header.Set("Range", "bytes=999-")
	w = httptest.NewRecorder()
	a.media(w, r)
	if w.Code != 416 {
		t.Fatal(w.Code)
	}
}
func TestRecordingMultipartAndTranscription(t *testing.T) {
	a := testApp(t)
	v := event("recording", "audio")
	data := []byte("RIFFmock audio")
	v["size"] = float64(len(data))
	v["fileName"] = "test.m4a"
	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	meta, _ := json.Marshal(v)
	mw.WriteField("metadata", string(meta))
	file, _ := mw.CreateFormFile("file", "recording.bin")
	file.Write(data)
	mw.Close()
	w := upload(a, "/v1/recordings", str(v["id"]), b.Bytes(), mw.FormDataContentType())
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	ff := filepath.Join(a.dir, "fakeffmpeg")
	wh := filepath.Join(a.dir, "fakewhisper")
	os.WriteFile(ff, []byte("#!/bin/sh\nfor last; do :; done\nprintf 'RIFFtest' > \"$last\"\n"), 0700)
	os.WriteFile(wh, []byte("#!/bin/sh\nwhile [ $# -gt 0 ]; do if [ \"$1\" = '-of' ]; then shift; printf 'Test transcript' > \"$1.txt\"; fi; shift; done\n"), 0700)
	a.cfg.FFmpeg = ff
	a.cfg.Whisper = wh
	a.cfg.Model = "fixture"
	if e := a.queueTranscripts(str(v["id"]), false); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { a.worker(ctx); close(done) }()
	defer func() { cancel(); <-done }()
	for i := 0; i < 100; i++ {
		r, _ := a.record(str(v["id"]))
		tr, _ := r["transcript"].(map[string]any)
		if tr["status"] == "done" {
			if tr["text"] != "Test transcript" {
				t.Fatal(tr)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("worker did not finish")
}
func TestMCPAndLocalBoundary(t *testing.T) {
	a := testApp(t)
	v := event("sms", "mcp")
	v["address"] = "010"
	sendEvent(t, a, v)
	for _, body := range []string{`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}`, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"search_messages","arguments":{"query":"hello"}}}`} {
		r := httptest.NewRequest("POST", "http://127.0.0.1:8789/mcp", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		private(8789, http.HandlerFunc(a.mcpHTTP)).ServeHTTP(w, r)
		if w.Code != 200 || !strings.Contains(w.Body.String(), "result") {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	for _, host := range []string{"evil.example:8788", "localhost:8788"} {
		r := httptest.NewRequest("GET", "http://"+host+"/api/connection", nil)
		if strings.HasPrefix(host, "localhost") {
			r.Header.Set("Origin", "https://evil.example")
		}
		w := httptest.NewRecorder()
		private(8788, http.HandlerFunc(a.viewer)).ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal(w.Code)
		}
	}
	if a.rpc([]byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)) != nil {
		t.Fatal("notification got reply")
	}
	r, e := a.callTool("get_conversation", map[string]any{"room": "010"})
	if e != nil || len(r.(map[string]any)["events"].([]map[string]any)) != 1 {
		t.Fatal(r, e)
	}
}
func TestBackupRestore(t *testing.T) {
	a := testApp(t)
	v := event("sms", "backup")
	sendEvent(t, a, v)
	out := filepath.Join(t.TempDir(), "backup")
	if e := transfer(a.dir, a.cfg, "backup", out, ""); e != nil {
		t.Fatal(e)
	}
	dest := t.TempDir()
	c, e := config(dest, true)
	if e != nil {
		t.Fatal(e)
	}
	if e = transfer(dest, c, "restore", "", out); e != nil {
		t.Fatal(e)
	}
	db, e := openDB(dest)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	var n int
	db.QueryRow("SELECT count(*) FROM events").Scan(&n)
	if n != 1 {
		t.Fatal(n)
	}
	if e = transfer(dest, c, "restore", "", out); e == nil {
		t.Fatal("overwrote existing data")
	}
}
