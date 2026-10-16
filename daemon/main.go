package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	_ "modernc.org/sqlite"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

const version = "1.1.0"

type Config struct {
	WebPort        int    `json:"webPort"`
	IngestPort     int    `json:"ingestPort"`
	MCPPort        int    `json:"mcpPort"`
	UploadKey      string `json:"uploadKey"`
	AdminKey       string `json:"adminKey"`
	PublicURL      string `json:"publicUrl"`
	FFmpeg         string `json:"ffmpeg"`
	Whisper        string `json:"whisper"`
	Model          string `json:"model"`
	Language       string `json:"language"`
	AutoTranscribe bool   `json:"autoTranscribe"`
	Tunnel         bool   `json:"quickTunnel"`
	Cloudflared    string `json:"cloudflared"`
	TunnelProtocol string `json:"tunnelProtocol"`
}
type App struct {
	dir      string
	cfg      Config
	db       *sql.DB
	started  time.Time
	stop     chan struct{}
	stopOnce sync.Once
}

func token() string {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func digest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func output(v any)           { b, _ := json.MarshalIndent(v, "", "  "); fmt.Println(string(b)) }
func must(e error) {
	if e != nil {
		fmt.Fprintln(os.Stderr, "pocketrelay:", e)
		os.Exit(1)
	}
}
func atomicWrite(path string, b []byte) error {
	f, e := os.CreateTemp(filepath.Dir(path), ".write-")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(b)
	}
	if e == nil {
		e = f.Sync()
	}
	f.Close()
	if e != nil {
		return e
	}
	return os.Rename(f.Name(), path)
}
func saveConfig(dir string, c Config) error {
	b, _ := json.MarshalIndent(c, "", "  ")
	return atomicWrite(filepath.Join(dir, "config.json"), b)
}
func config(dir string, create bool) (Config, error) {
	var c Config
	b, e := os.ReadFile(filepath.Join(dir, "config.json"))
	if os.IsNotExist(e) && create {
		if e = os.MkdirAll(dir, 0700); e != nil {
			return c, e
		}
		c = Config{WebPort: 8788, IngestPort: 8787, MCPPort: 8789, UploadKey: token(), AdminKey: token(), FFmpeg: "ffmpeg", Whisper: "whisper-cli", Language: "ko", Cloudflared: "cloudflared"}
		return c, saveConfig(dir, c)
	}
	if e != nil {
		return c, e
	}
	e = json.Unmarshal(b, &c)
	return c, e
}
func openDB(dir string) (*sql.DB, error) {
	d, e := sql.Open("sqlite", filepath.Join(dir, "events.sqlite3"))
	if e != nil {
		return nil, e
	}
	d.SetMaxOpenConns(1)
	_, e = d.Exec(`PRAGMA busy_timeout=10000; PRAGMA journal_mode=WAL; CREATE TABLE IF NOT EXISTS events(id TEXT PRIMARY KEY,payload TEXT NOT NULL,audio TEXT); CREATE TABLE IF NOT EXISTS transcripts(id TEXT PRIMARY KEY,text TEXT NOT NULL DEFAULT '',status TEXT NOT NULL,updated INTEGER NOT NULL,error TEXT NOT NULL DEFAULT '');`)
	if e != nil {
		d.Close()
		return nil, e
	}
	return d, nil
}
func admin(c Config, path string, method string) (map[string]any, error) {
	req, _ := http.NewRequest(method, fmt.Sprintf("http://127.0.0.1:%d/api/control/%s", c.WebPort, path), nil)
	req.Header.Set("Authorization", "Bearer "+c.AdminKey)
	client := http.Client{Timeout: 3 * time.Second}
	r, e := client.Do(req)
	if e != nil {
		return nil, e
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return nil, fmt.Errorf("control HTTP %d", r.StatusCode)
	}
	var result map[string]any
	e = json.NewDecoder(r.Body).Decode(&result)
	return result, e
}
func start(dir string, c Config) error {
	if s, e := admin(c, "state", "GET"); e == nil {
		output(s)
		return nil
	}
	if managed(dir) {
		if e := systemctl("start", "pocketrelay.service"); e != nil {
			return e
		}
		for i := 0; i < 60; i++ {
			time.Sleep(100 * time.Millisecond)
			if s, e := admin(c, "state", "GET"); e == nil {
				output(s)
				return nil
			}
		}
		return errors.New("service did not become ready; inspect journalctl --user -u pocketrelay")
	}
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	log, e := os.OpenFile(filepath.Join(dir, "daemon.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	defer log.Close()
	cmd := exec.Command(exe, "serve", "--data", dir)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdin = nil
	cmd.Stdout = log
	cmd.Stderr = log
	if e = cmd.Start(); e != nil {
		return e
	}
	cmd.Process.Release()
	for i := 0; i < 60; i++ {
		time.Sleep(100 * time.Millisecond)
		if s, e := admin(c, "state", "GET"); e == nil {
			output(s)
			return nil
		}
	}
	return errors.New("daemon did not start; run pocketrelay logs or doctor")
}
func stop(c Config) error {
	_, e := admin(c, "stop", "POST")
	if e != nil {
		return e
	}
	for i := 0; i < 50; i++ {
		time.Sleep(100 * time.Millisecond)
		if _, e = admin(c, "state", "GET"); e != nil {
			return nil
		}
	}
	return errors.New("daemon still shutting down")
}
func links(dir string, c Config) map[string]any {
	external := c.PublicURL
	if c.Tunnel {
		if b, e := os.ReadFile(filepath.Join(dir, "tunnel-url")); e == nil {
			external = strings.TrimSpace(string(b))
		}
	}
	return map[string]any{"web": fmt.Sprintf("http://localhost:%d", c.WebPort), "mcp": fmt.Sprintf("http://127.0.0.1:%d/mcp", c.MCPPort), "phoneServer": external, "phoneServerReady": external != "", "note": "localhost web/MCP are private. The phone needs HTTPS to the ingest port."}
}

const help = `PocketRelay 1.1 — Ubuntu daemon, local web UI and read-only Dot MCP

pocketrelay [start]             Start in background (default)
pocketrelay stop | restart      Stop / restart
pocketrelay --state [--json]    Status (aliases: status, --status, --sate)
pocketrelay --link              Local web, phone HTTPS and MCP URLs
pocketrelay --pair              QR setup link, phone server and upload key (alias: pair)
pocketrelay --check-link        Test public HTTPS access and upload authentication
pocketrelay config --tunnel-protocol http2   Use TCP tunnel when UDP is unavailable
pocketrelay logs [--lines 80]   Recent logs
pocketrelay doctor             Environment and port checks
pocketrelay config             Show redacted configuration
pocketrelay config --url URL --web-port N --ingest-port N --mcp-port N
pocketrelay tunnel on|off       Optional Cloudflare quick tunnel to ingest ONLY
pocketrelay query --type sms --query TEXT --after 0 --limit 50
pocketrelay get --id ID         Retrieve one saved record and transcript
pocketrelay export --out PATH   Export all events as JSONL
pocketrelay backup --out DIR    Consistent DB + originals backup (service stopped)
pocketrelay restore --from DIR  Restore into an empty data directory
pocketrelay migrate --from DIR Import existing receiver-data (empty destination)
pocketrelay transcribe --id ID  Queue recording, or --all
pocketrelay config --whisper PATH --model PATH --ffmpeg PATH --auto-transcribe=true
pocketrelay mcp                 Read-only MCP over stdio for local clients
pocketrelay install-service    Install/enable user systemd service
pocketrelay uninstall-service  Remove service, keep all data
pocketrelay serve              Foreground supervisor mode (systemd)
pocketrelay --version | --help

All commands: --data DIR (default ~/.local/share/pocketrelay), --json
Nothing transcribes or connects to ChatGPT until configured. Web: localhost:8788.
`

func main() {
	args := os.Args[1:]
	command := "start"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		command = args[0]
		args = args[1:]
	}
	if command == "tunnel" && len(args) > 0 && (args[0] == "on" || args[0] == "off") {
		args = append(args[1:], args[0])
	}
	home, _ := os.UserHomeDir()
	f := flag.NewFlagSet("pocketrelay", flag.ContinueOnError)
	f.SetOutput(os.Stderr)
	dir := f.String("data", filepath.Join(home, ".local", "share", "pocketrelay"), "data directory")
	js := f.Bool("json", false, "JSON output")
	_ = js
	state := f.Bool("state", false, "status")
	status := f.Bool("status", false, "status")
	sate := f.Bool("sate", false, "status alias")
	link := f.Bool("link", false, "URLs")
	pairFlag := f.Bool("pair", false, "QR setup link, phone server and upload key")
	checkLink := f.Bool("check-link", false, "verify public HTTPS and authentication")
	protocol := f.String("tunnel-protocol", "", "auto, http2 or quic")
	h := f.Bool("help", false, "help")
	v := f.Bool("version", false, "version")
	urlFlag := f.String("url", "", "stable HTTPS ingest URL")
	web := f.Int("web-port", 0, "web port")
	ingest := f.Int("ingest-port", 0, "ingest port")
	mcp := f.Int("mcp-port", 0, "MCP port")
	q := f.String("query", "", "search text")
	kind := f.String("type", "", "event type")
	after := f.Int64("after", 0, "cursor")
	limit := f.Int("limit", 50, "page size")
	id := f.String("id", "", "event id")
	out := f.String("out", "", "output path")
	from := f.String("from", "", "source directory")
	lines := f.Int("lines", 80, "log lines")
	all := f.Bool("all", false, "all recordings")
	whisper := f.String("whisper", "", "whisper-cli path")
	model := f.String("model", "", "ggml model path")
	ffmpeg := f.String("ffmpeg", "", "ffmpeg path")
	cloudflared := f.String("cloudflared", "", "cloudflared executable path")
	automatic := f.String("auto-transcribe", "", "true or false")
	language := f.String("language", "", "transcription language")
	must(f.Parse(args))
	if *h || command == "help" {
		fmt.Print(help)
		return
	}
	if *v || command == "version" {
		fmt.Println(version)
		return
	}
	if *state || *status || *sate {
		command = "status"
	}
	if *link {
		command = "links"
	}
	if *pairFlag {
		command = "pair"
	}
	if *checkLink {
		command = "check-link"
	}
	*dir, _ = filepath.Abs(*dir)
	c, e := config(*dir, command == "init" || command == "start" || command == "config" || command == "restore" || command == "migrate")
	must(e)
	switch command {
	case "init":
		output(map[string]any{"initialized": true, "data": *dir})
	case "start":
		must(start(*dir, c))
	case "stop":
		must(stop(c))
		output(map[string]any{"running": false})
	case "restart":
		if _, e := admin(c, "state", "GET"); e == nil {
			must(stop(c))
		}
		must(start(*dir, c))
	case "status":
		s, e := admin(c, "state", "GET")
		if e != nil {
			output(map[string]any{"running": false, "data": *dir})
			return
		}
		output(s)
	case "links":
		output(links(*dir, c))
	case "pair":
		output(pairing(*dir, c))
	case "check-link":
		output(checkPublic(*dir, c))
	case "logs":
		if managed(*dir) {
			cmd := exec.Command("journalctl", "--user", "-u", "pocketrelay", "-n", fmt.Sprint(*lines), "--no-pager")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			must(cmd.Run())
			return
		}
		b, e := os.ReadFile(filepath.Join(*dir, "daemon.log"))
		must(e)
		parts := strings.Split(strings.TrimSpace(string(b)), "\n")
		n := *lines
		if n < 1 {
			n = 80
		}
		if len(parts) > n {
			parts = parts[len(parts)-n:]
		}
		fmt.Println(strings.Join(parts, "\n"))
	case "doctor":
		output(doctor(*dir, c))
	case "config":
		changed := false
		f.Visit(func(fl *flag.Flag) {
			if fl.Name != "data" && fl.Name != "json" {
				changed = true
			}
		})
		if *urlFlag != "" {
			if !strings.HasPrefix(*urlFlag, "https://") {
				must(errors.New("phone URL must use HTTPS"))
			}
			c.PublicURL = strings.TrimRight(*urlFlag, "/")
		}
		if *web != 0 {
			c.WebPort = *web
		}
		if *ingest != 0 {
			c.IngestPort = *ingest
		}
		if *mcp != 0 {
			c.MCPPort = *mcp
		}
		if *whisper != "" {
			c.Whisper = *whisper
		}
		if *model != "" {
			c.Model = *model
		}
		if *ffmpeg != "" {
			c.FFmpeg = *ffmpeg
		}
		if *cloudflared != "" {
			c.Cloudflared = *cloudflared
		}
		if *protocol != "" {
			if *protocol != "auto" && *protocol != "http2" && *protocol != "quic" {
				must(errors.New("tunnel-protocol must be auto, http2 or quic"))
			}
			c.TunnelProtocol = *protocol
		}
		if *language != "" {
			c.Language = *language
		}
		if *automatic != "" {
			if *automatic != "true" && *automatic != "false" {
				must(errors.New("auto-transcribe must be true/false"))
			}
			c.AutoTranscribe = *automatic == "true"
		}
		for _, p := range []int{c.WebPort, c.IngestPort, c.MCPPort} {
			if p < 1024 || p > 65535 {
				must(errors.New("port must be 1024..65535"))
			}
		}
		if c.WebPort == c.IngestPort || c.WebPort == c.MCPPort || c.MCPPort == c.IngestPort {
			must(errors.New("ports must differ"))
		}
		if changed {
			must(saveConfig(*dir, c))
		}
		c.UploadKey = "(use pair)"
		c.AdminKey = "(private)"
		output(map[string]any{"config": c, "restartRequired": changed})
	case "tunnel":
		if f.NArg() != 1 || (f.Arg(0) != "on" && f.Arg(0) != "off") {
			must(errors.New("use tunnel on or tunnel off"))
		}
		c.Tunnel = f.Arg(0) == "on"
		must(saveConfig(*dir, c))
		if _, e := admin(c, "state", "GET"); e == nil {
			must(stop(c))
			must(start(*dir, c))
		} else {
			output(map[string]any{"quickTunnel": c.Tunnel})
		}
	case "serve":
		must(serve(*dir, c))
	case "install-service", "uninstall-service":
		must(service(*dir, command == "install-service"))
	case "backup", "restore", "migrate":
		must(transfer(*dir, c, command, *out, *from))
	default:
		db, e := openDB(*dir)
		must(e)
		defer db.Close()
		a := &App{dir: *dir, cfg: c, db: db}
		switch command {
		case "query":
			r, e := a.query(*kind, *q, *after, *limit)
			must(e)
			output(r)
		case "get":
			r, e := a.record(*id)
			must(e)
			output(r)
		case "export":
			if *out == "" {
				must(errors.New("--out required"))
			}
			file, e := os.OpenFile(*out, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			must(e)
			defer file.Close()
			rows, e := db.Query("SELECT payload FROM events ORDER BY rowid")
			must(e)
			defer rows.Close()
			for rows.Next() {
				var b string
				must(rows.Scan(&b))
				_, e = fmt.Fprintln(file, b)
				must(e)
			}
			must(rows.Err())
			output(map[string]any{"exported": *out})
		case "transcribe":
			must(a.queueTranscripts(*id, *all))
			output(map[string]any{"queued": true, "workerNeedsRunningDaemon": true})
		case "mcp":
			must(a.stdio())
		default:
			must(errors.New("unknown command; use --help"))
		}
	}
}
func copyFile(src, dst string) error {
	in, e := os.Open(src)
	if e != nil {
		return e
	}
	defer in.Close()
	out, e := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	_, e = io.Copy(out, in)
	if e == nil {
		e = out.Sync()
	}
	out.Close()
	return e
}
func postJSON(url string, key string, v any) error {
	b, _ := json.Marshal(v)
	r, _ := http.NewRequest("POST", url, bytes.NewReader(b))
	r.Header.Set("Authorization", "Bearer "+key)
	r.Header.Set("Content-Type", "application/json")
	res, e := http.DefaultClient.Do(r)
	if e != nil {
		return e
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("HTTP %d", res.StatusCode)
	}
	return nil
}
