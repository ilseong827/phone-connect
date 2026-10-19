package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

const recordingChunkSize = 8 << 20
const recordingMaxSize = 1 << 30

var recordingUploadLock sync.Mutex

// Each acknowledged chunk is atomically stored on the data mount. No session
// state is held in memory, so an app or server restart can resume the upload.
func (a *App) recordingUpload(w http.ResponseWriter, r *http.Request, id string) {
	recordingUploadLock.Lock()
	defer recordingUploadLock.Unlock()
	action := r.URL.Path
	if action != "/v1/recording-upload/start" && action != "/v1/recording-upload/chunk" && action != "/v1/recording-upload/complete" {
		fail(w, 404, "Not found")
		return
	}
	dir := filepath.Join(a.dir, "recording-uploads", id)
	var exists int
	if a.db.QueryRow("SELECT 1 FROM events WHERE id=?", id).Scan(&exists) == nil {
		os.RemoveAll(dir)
		respond(w, 200, map[string]any{"accepted": true, "id": id})
		return
	}
	var event map[string]any
	var meta []byte
	var err error
	if action == "/v1/recording-upload/start" {
		meta, err = io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	} else {
		meta, err = os.ReadFile(filepath.Join(dir, "metadata.json"))
	}
	if err != nil || json.Unmarshal(meta, &event) != nil {
		fail(w, 400, "Missing or invalid upload metadata")
		return
	}
	size, ok := event["size"].(float64)
	if !ok || size <= 0 || size > recordingMaxSize || size != float64(int64(size)) || event["id"] != id || event["type"] != "recording" || event["schemaVersion"] != float64(1) {
		fail(w, 400, "Invalid recording metadata (maximum 1 GiB)")
		return
	}
	total := int64(size)
	count := int((total + recordingChunkSize - 1) / recordingChunkSize)
	if action == "/v1/recording-upload/start" {
		canonical, _ := json.Marshal(event)
		if old, e := os.ReadFile(filepath.Join(dir, "metadata.json")); e == nil && string(old) != string(canonical) {
			fail(w, 409, "Upload metadata changed")
			return
		}
		if os.MkdirAll(dir, 0700) != nil || atomicWrite(filepath.Join(dir, "metadata.json"), canonical) != nil {
			fail(w, 500, "Storage error")
			return
		}
	}
	chunkPath := func(i int) string { return filepath.Join(dir, strconv.Itoa(i)+".part") }
	expected := func(i int) int64 {
		n := total - int64(i)*recordingChunkSize
		if n > recordingChunkSize {
			return recordingChunkSize
		}
		return n
	}
	if action == "/v1/recording-upload/chunk" {
		i, e := strconv.Atoi(r.URL.Query().Get("index"))
		if e != nil || i < 0 || i >= count {
			fail(w, 400, "Invalid chunk index")
			return
		}
		b, e := io.ReadAll(http.MaxBytesReader(w, r.Body, recordingChunkSize))
		if e != nil || int64(len(b)) != expected(i) || digest(b) != r.Header.Get("X-Chunk-SHA256") {
			fail(w, 400, "Chunk size or checksum mismatch")
			return
		}
		if old, e := os.ReadFile(chunkPath(i)); e == nil && digest(old) != digest(b) {
			fail(w, 409, "Chunk differs from acknowledged data")
			return
		}
		if atomicWrite(chunkPath(i), b) != nil {
			fail(w, 500, "Storage error")
			return
		}
	}
	next := count
	for i := 0; i < count; i++ {
		s, e := os.Stat(chunkPath(i))
		if e != nil || s.Size() != expected(i) {
			next = i
			break
		}
	}
	if action != "/v1/recording-upload/complete" {
		respond(w, 200, map[string]any{"id": id, "nextChunk": next, "chunkSize": recordingChunkSize})
		return
	}
	if next != count {
		fail(w, 409, "Upload incomplete")
		return
	}
	f, e := os.CreateTemp(a.dir, ".assembled-")
	if e != nil {
		fail(w, 500, "Storage error")
		return
	}
	defer os.Remove(f.Name())
	defer f.Close()
	for i := 0; i < count; i++ {
		src, e := os.Open(chunkPath(i))
		if e != nil {
			fail(w, 500, "Missing chunk")
			return
		}
		n, e := io.Copy(f, src)
		src.Close()
		if e != nil || n != expected(i) {
			fail(w, 500, "Assembly failed")
			return
		}
	}
	if f.Sync() != nil || f.Close() != nil {
		fail(w, 500, "Storage error")
		return
	}
	if e = a.accept(event, f.Name()); e != nil {
		fail(w, 500, "Recording save failed")
		return
	}
	os.RemoveAll(dir)
	respond(w, 200, map[string]any{"accepted": true, "id": id})
}
