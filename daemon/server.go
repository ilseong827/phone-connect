package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

//go:embed web/*
var webFiles embed.FS
var validID = regexp.MustCompile(`^[a-f0-9]{64}$`)
var mediaLock sync.Mutex
var acceptLock sync.Mutex

func str(v any) string { s, _ := v.(string); return s }
func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, s string) {
	respond(w, status, map[string]any{"error": s})
}
func authorized(r *http.Request, key string) bool {
	return subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+key)) == 1
}
func private(port int, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allowed := func(s string) bool {
			return s == fmt.Sprintf("localhost:%d", port) || s == fmt.Sprintf("127.0.0.1:%d", port)
		}
		origin := r.Header.Get("Origin")
		if !allowed(r.Host) || (origin != "" && !allowed(strings.TrimPrefix(origin, "http://"))) {
			fail(w, 403, "Local access only")
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
func serve(dir string, c Config) error {
	lock, e := os.OpenFile(filepath.Join(dir, "daemon.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return e
	}
	defer lock.Close()
	if e = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		return errors.New("daemon already owns data directory")
	}
	db, e := openDB(dir)
	if e != nil {
		return e
	}
	defer db.Close()
	a := &App{dir: dir, cfg: c, db: db, started: time.Now(), stop: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	handlers := []http.Handler{http.HandlerFunc(a.ingest), private(c.WebPort, http.HandlerFunc(a.viewer)), private(c.MCPPort, http.HandlerFunc(a.mcpHTTP))}
	ports := []int{c.IngestPort, c.WebPort, c.MCPPort}
	listeners := []net.Listener{}
	defer func() {
		for _, l := range listeners {
			l.Close()
		}
	}()
	for _, p := range ports {
		l, e := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if e != nil {
			return e
		}
		listeners = append(listeners, l)
	}
	servers := []*http.Server{}
	errs := make(chan error, 3)
	for i, l := range listeners {
		s := &http.Server{Handler: handlers[i], ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 10 * time.Minute, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32768}
		servers = append(servers, s)
		go func(s *http.Server, l net.Listener) { errs <- s.Serve(l) }(s, l)
	}
	go a.worker(ctx)
	if c.Tunnel {
		go a.tunnel(ctx)
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)
	fmt.Println(time.Now().UTC().Format(time.RFC3339), "PocketRelay ready", version)
	select {
	case <-a.stop:
	case <-signals:
	case e = <-errs:
	}
	cancel()
	shutdown, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()
	for _, s := range servers {
		s.Shutdown(shutdown)
	}
	return e
}
func (a *App) state() map[string]any {
	var n, t int
	a.db.QueryRow("SELECT count(*) FROM events").Scan(&n)
	a.db.QueryRow("SELECT count(*) FROM transcripts WHERE status='done'").Scan(&t)
	return map[string]any{"running": true, "pid": os.Getpid(), "version": version, "data": a.dir, "started": a.started, "records": n, "transcribed": t, "links": links(a.dir, a.cfg)}
}
func (a *App) ingest(w http.ResponseWriter, r *http.Request) {
	if pairingAsset(w, r) {
		return
	}
	if r.URL.Path == "/v1/pair/check" && r.Method == "POST" {
		if !authorized(r, a.cfg.UploadKey) {
			fail(w, 401, "Unauthorized")
			return
		}
		respond(w, 200, map[string]any{"ok": true, "service": "Phone Connect", "version": version})
		return
	}
	if r.Method == "GET" && r.URL.Path == "/health" {
		respond(w, 200, map[string]any{"ok": true, "service": "PocketRelay"})
		return
	}
	if r.Method == "GET" && r.URL.Path == "/app.apk" {
		exe, _ := os.Executable()
		http.ServeFile(w, r, filepath.Join(filepath.Dir(exe), "PocketRelay-debug.apk"))
		return
	}
	if r.Method != "POST" {
		fail(w, 404, "Not found")
		return
	}
	if !authorized(r, a.cfg.UploadKey) {
		fail(w, 401, "Unauthorized")
		return
	}
	id := r.Header.Get("Idempotency-Key")
	if !validID.MatchString(id) {
		fail(w, 400, "Invalid id")
		return
	}
	if r.URL.Path == "/v1/attachments/"+id {
		f, e := os.CreateTemp(a.dir, ".upload-")
		if e != nil {
			fail(w, 500, "Storage error")
			return
		}
		defer os.Remove(f.Name())
		defer f.Close()
		hash := sha256.New()
		n, e := io.Copy(io.MultiWriter(f, hash), http.MaxBytesReader(w, r.Body, 100<<20))
		if e != nil || n == 0 || hex.EncodeToString(hash.Sum(nil)) != id {
			fail(w, 400, "Invalid attachment or checksum")
			return
		}
		if e = f.Sync(); e == nil {
			e = f.Close()
		}
		if e == nil {
			e = os.Rename(f.Name(), filepath.Join(a.dir, id+".blob"))
		}
		if e != nil {
			fail(w, 500, "Storage error")
			return
		}
		respond(w, 200, map[string]any{"accepted": true, "id": id})
		return
	}
	var event map[string]any
	audio := ""
	switch r.URL.Path {
	case "/v1/events":
		if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 100<<20)).Decode(&event); e != nil {
			fail(w, 400, "Invalid JSON")
			return
		}
	case "/v1/recordings":
		r.Body = http.MaxBytesReader(w, r.Body, 512<<20)
		if e := r.ParseMultipartForm(1 << 20); e != nil {
			fail(w, 400, "Invalid recording")
			return
		}
		defer r.MultipartForm.RemoveAll()
		if e := json.Unmarshal([]byte(r.FormValue("metadata")), &event); e != nil {
			fail(w, 400, "Invalid metadata")
			return
		}
		src, _, e := r.FormFile("file")
		if e != nil {
			fail(w, 400, "Missing recording")
			return
		}
		defer src.Close()
		f, e := os.CreateTemp(a.dir, ".audio-")
		if e != nil {
			fail(w, 500, "Storage error")
			return
		}
		defer os.Remove(f.Name())
		n, e := io.Copy(f, src)
		f.Sync()
		f.Close()
		if e != nil || float64(n) != event["size"] {
			fail(w, 400, "Recording size mismatch")
			return
		}
		audio = f.Name()
	default:
		fail(w, 404, "Not found")
		return
	}
	if event == nil || event["id"] != id || event["schemaVersion"] != float64(1) {
		fail(w, 400, "Invalid event")
		return
	}
	kind := str(event["type"])
	validKind := map[string]bool{"kakao": true, "sms": true, "recording": true, "file": true, "contacts": true}
	if !validKind[kind] || (kind == "recording") != (audio != "") {
		fail(w, 400, "Invalid event type")
		return
	}
	if e := a.accept(event, audio); e != nil {
		fail(w, 400, "Invalid attachment or storage failure")
		return
	}
	respond(w, 200, map[string]any{"accepted": true, "id": id})
}
func (a *App) accept(event map[string]any, audio string) error {
	acceptLock.Lock()
	defer acceptLock.Unlock()
	id := str(event["id"])
	var exists int
	if a.db.QueryRow("SELECT 1 FROM events WHERE id=?", id).Scan(&exists) == nil {
		return nil
	}
	list, _ := event["attachments"].([]any)
	if len(list) > 100 {
		return errors.New("too many attachments")
	}
	for _, v := range list {
		item, ok := v.(map[string]any)
		if !ok {
			return errors.New("bad attachment")
		}
		delete(item, "localFile")
		if encoded, ok := item["data"].(string); ok {
			b, e := base64.StdEncoding.DecodeString(encoded)
			if e != nil || len(b) == 0 || len(b) > 8<<20 {
				return errors.New("bad preview")
			}
			fid := digest(b)
			if e = atomicWrite(filepath.Join(a.dir, fid+".blob"), b); e != nil {
				return e
			}
			item["fileId"] = fid
			item["imageId"] = fid
			item["size"] = float64(len(b))
			delete(item, "data")
		}
		if fid := str(item["fileId"]); fid != "" {
			if !validID.MatchString(fid) {
				return errors.New("invalid file id")
			}
			p := filepath.Join(a.dir, fid+".blob")
			st, e := os.Stat(p)
			if e != nil || float64(st.Size()) != item["size"] {
				return errors.New("upload attachment first")
			}
			item["mimeType"] = sniff(p)
			delete(item, "unavailable")
		}
	}
	var saved any
	if audio != "" {
		saved = id + ".audio"
		if e := os.Rename(audio, filepath.Join(a.dir, saved.(string))); e != nil {
			return e
		}
	}
	b, e := json.Marshal(event)
	if e != nil {
		return e
	}
	_, e = a.db.Exec("INSERT OR IGNORE INTO events(id,payload,audio) VALUES(?,?,?)", id, string(b), saved)
	if e == nil && audio != "" && a.cfg.AutoTranscribe {
		e = a.queueTranscripts(id, false)
	}
	return e
}
func sniff(path string) string {
	f, e := os.Open(path)
	if e != nil {
		return "application/octet-stream"
	}
	defer f.Close()
	b := make([]byte, 512)
	n, _ := f.Read(b)
	b = b[:n]
	if n >= 12 && string(b[4:8]) == "ftyp" {
		return "video/mp4"
	}
	return http.DetectContentType(b)
}
func (a *App) viewer(w http.ResponseWriter, r *http.Request) {
	if pairingAsset(w, r) {
		return
	}
	p := r.URL.Path
	if strings.HasPrefix(p, "/api/control/") {
		if !authorized(r, a.cfg.AdminKey) {
			fail(w, 401, "Unauthorized")
			return
		}
		if p == "/api/control/state" && r.Method == "GET" {
			respond(w, 200, a.state())
			return
		}
		if p == "/api/control/stop" && r.Method == "POST" {
			respond(w, 200, map[string]bool{"stopping": true})
			a.stopOnce.Do(func() { close(a.stop) })
			return
		}
		fail(w, 404, "Not found")
		return
	}
	if r.Method != "GET" && r.Method != "HEAD" {
		fail(w, 405, "Read only")
		return
	}
	switch p {
	case "/", "/chat.js":
		name := "web/chat.html"
		if p != "/" {
			name = "web/chat.js"
		}
		b, _ := webFiles.ReadFile(name)
		w.Header().Set("Content-Type", mime.TypeByExtension(filepath.Ext(name)))
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'unsafe-inline'; script-src 'self'; frame-ancestors 'none'; base-uri 'none'")
		w.Write(b)
	case "/api/events":
		after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
		v, e := a.query("", "", after, 500)
		if e != nil {
			fail(w, 500, "Database error")
			return
		}
		respond(w, 200, v)
	case "/api/connection":
		l := links(a.dir, a.cfg)
		respond(w, 200, map[string]any{"url": l["phoneServer"], "token": a.cfg.UploadKey})
	case "/api/state":
		respond(w, 200, a.state())
	default:
		if strings.HasPrefix(p, "/api/record/") {
			v, e := a.record(strings.TrimPrefix(p, "/api/record/"))
			if e != nil {
				fail(w, 404, "Not found")
				return
			}
			respond(w, 200, v)
			return
		}
		if strings.HasPrefix(p, "/audio/") || strings.HasPrefix(p, "/file/") || strings.HasPrefix(p, "/image/") {
			a.media(w, r)
			return
		}
		fail(w, 404, "Not found")
	}
}
func (a *App) media(w http.ResponseWriter, r *http.Request) {
	id := filepath.Base(r.URL.Path)
	if !validID.MatchString(id) {
		fail(w, 404, "Not found")
		return
	}
	path := ""
	name := "attachment"
	inline := false
	if strings.HasPrefix(r.URL.Path, "/audio/") {
		v, e := a.record(id)
		if e != nil || v["type"] != "recording" {
			fail(w, 404, "Not found")
			return
		}
		path = filepath.Join(a.dir, id+".audio")
		name = str(v["fileName"])
		inline = r.URL.Query().Get("download") != "1"
		if r.URL.Query().Get("format") == "wav" {
			var e error
			path, e = a.wav(r.Context(), id)
			if e != nil {
				fail(w, 503, "Playback conversion unavailable; run pocketrelay doctor and install FFmpeg")
				return
			}
			name = "recording.wav"
		}
	} else {
		path = filepath.Join(a.dir, id+".blob")
		if _, e := os.Stat(path); e != nil {
			path = filepath.Join(a.dir, id+".image")
		}
		mimeType := sniff(path)
		inline = (r.URL.Query().Get("preview") == "1" || strings.HasPrefix(r.URL.Path, "/image/")) && (strings.HasPrefix(mimeType, "image/") && mimeType != "image/svg+xml" || strings.HasPrefix(mimeType, "audio/") || strings.HasPrefix(mimeType, "video/"))
		rows, e := a.db.Query("SELECT payload FROM events WHERE payload LIKE ?", "%"+id+"%")
		if e == nil {
			for rows.Next() {
				var b string
				rows.Scan(&b)
				var v map[string]any
				json.Unmarshal([]byte(b), &v)
				ls, _ := v["attachments"].([]any)
				for _, x := range ls {
					item, _ := x.(map[string]any)
					if item["fileId"] == id || item["imageId"] == id {
						name = str(item["fileName"])
					}
				}
			}
			rows.Close()
		}
	}
	f, e := os.Open(path)
	if e != nil {
		fail(w, 404, "Not found")
		return
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil {
		fail(w, 500, "File error")
		return
	}
	disposition := "attachment"
	ct := "application/octet-stream"
	if inline {
		disposition = "inline"
		ct = sniff(path)
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": filepath.Base(name)}))
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
	http.ServeContent(w, r, name, st.ModTime(), f)
}
func (a *App) wav(ctx context.Context, id string) (string, error) {
	mediaLock.Lock()
	defer mediaLock.Unlock()
	dest := filepath.Join(a.dir, id+".playback.wav")
	if st, e := os.Stat(dest); e == nil && st.Size() > 44 {
		return dest, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	temp := dest + ".tmp"
	defer os.Remove(temp)
	cmd := exec.CommandContext(ctx, a.cfg.FFmpeg, "-nostdin", "-v", "error", "-y", "-i", filepath.Join(a.dir, id+".audio"), "-vn", "-c:a", "pcm_s16le", "-f", "wav", temp)
	if e := cmd.Run(); e != nil {
		return "", e
	}
	return dest, os.Rename(temp, dest)
}
