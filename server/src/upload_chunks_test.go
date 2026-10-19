package main

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestResumableRecording(t *testing.T) {
	a := testApp(t)
	data := bytes.Repeat([]byte("recording"), recordingChunkSize/9+9)
	v := event("recording", "chunks")
	v["size"] = float64(len(data))
	v["fileName"] = "test.m4a"
	id := str(v["id"])
	meta, _ := json.Marshal(v)
	request := func(action string, b []byte, hash string, auth bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/v1/recording-upload/"+action, bytes.NewReader(b))
		r.Header.Set("Idempotency-Key", id)
		r.Header.Set("X-Chunk-SHA256", hash)
		if auth {
			r.Header.Set("Authorization", "Bearer "+a.cfg.UploadKey)
		}
		w := httptest.NewRecorder()
		a.ingest(w, r)
		return w
	}
	check := func(w *httptest.ResponseRecorder, code int) {
		t.Helper()
		if w.Code != code {
			t.Fatalf("%d: %s", w.Code, w.Body.String())
		}
	}
	check(request("start", meta, "", false), 401)
	check(request("start", meta, "", true), 200)
	check(request("complete", nil, "", true), 409)
	check(request("chunk?index=-1", nil, "", true), 400)
	first := data[:recordingChunkSize]
	check(request("chunk?index=0", first, "bad", true), 400)
	check(request("chunk?index=0", first, digest(first), true), 200)
	check(request("chunk?index=0", first, digest(first), true), 200)
	changed := append([]byte(nil), first...)
	changed[0] ^= 1
	check(request("chunk?index=0", changed, digest(changed), true), 409)
	// Fresh App object simulates process state loss; progress must come from disk.
	a = &App{dir: a.dir, cfg: a.cfg, db: a.db}
	w := request("start", meta, "", true)
	check(w, 200)
	var state map[string]any
	json.Unmarshal(w.Body.Bytes(), &state)
	if state["nextChunk"] != float64(1) {
		t.Fatal(state)
	}
	rest := data[recordingChunkSize:]
	check(request("chunk?index=1", rest, digest(rest), true), 200)
	check(request("complete", nil, "", true), 200)
	check(request("complete", nil, "", true), 200)
	saved, e := os.ReadFile(filepath.Join(a.dir, id+".audio"))
	if e != nil || !bytes.Equal(saved, data) {
		t.Fatal("assembled recording differs", e)
	}
	var count int
	a.db.QueryRow("SELECT count(*) FROM events WHERE id=?", id).Scan(&count)
	if count != 1 {
		t.Fatal(count)
	}
	if _, e = os.Stat(filepath.Join(a.dir, "recording-uploads", id)); !os.IsNotExist(e) {
		t.Fatal("chunks not cleaned")
	}
}
