package main

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

func managed(dir string) bool {
	home, _ := os.UserHomeDir()
	b, e := os.ReadFile(filepath.Join(home, ".config", "systemd", "user", "pocketrelay.service"))
	return e == nil && strings.Contains(string(b), "--data \""+dir+"\"")
}

func doctor(dir string, c Config) map[string]any {
	checks := map[string]any{}
	for name, path := range map[string]string{"ffmpeg": c.FFmpeg, "whisper": c.Whisper, "cloudflared": c.Cloudflared, "systemctl": "systemctl"} {
		p, e := exec.LookPath(path)
		checks[name] = map[string]any{"available": e == nil, "path": p}
	}
	st, e := os.Stat(c.Model)
	checks["model"] = map[string]any{"configured": c.Model != "", "exists": e == nil && st != nil && !st.IsDir()}
	ports := map[string]any{}
	for name, p := range map[string]int{"web": c.WebPort, "ingest": c.IngestPort, "mcp": c.MCPPort} {
		l, e := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		ports[name] = map[string]any{"port": p, "free": e == nil}
		if e == nil {
			l.Close()
		}
	}
	_, e = admin(c, "state", "GET")
	return map[string]any{"checks": checks, "ports": ports, "daemonRunning": e == nil, "data": dir, "autoTranscribe": c.AutoTranscribe, "links": links(dir, c)}
}
func systemctl(args ...string) error {
	cmd := exec.Command("systemctl", append([]string{"--user"}, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
func service(dir string, install bool) error {
	home, e := os.UserHomeDir()
	if e != nil {
		return e
	}
	units := filepath.Join(home, ".config", "systemd", "user")
	path := filepath.Join(units, "pocketrelay.service")
	if !install {
		if e = systemctl("disable", "--now", "pocketrelay.service"); e != nil {
			return e
		}
		if e = os.Remove(path); e != nil && !os.IsNotExist(e) {
			return e
		}
		return systemctl("daemon-reload")
	}
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	for _, p := range []string{exe, dir} {
		if strings.ContainsAny(p, "\n\r%\"") {
			return errors.New("service path contains unsupported characters")
		}
	}
	c, e := config(dir, false)
	if e != nil {
		return e
	}
	if _, e = admin(c, "state", "GET"); e == nil {
		if e = stop(c); e != nil {
			return e
		}
	}
	if e = os.MkdirAll(units, 0700); e != nil {
		return e
	}
	unit := fmt.Sprintf("[Unit]\nDescription=PocketRelay personal records\nAfter=network-online.target\n\n[Service]\nType=simple\nExecStart=\"%s\" serve --data \"%s\"\nRestart=on-failure\nRestartSec=5\nTimeoutStopSec=30\nUMask=0077\nNoNewPrivileges=true\nPrivateTmp=true\n\n[Install]\nWantedBy=default.target\n", exe, dir)
	if e = atomicWrite(path, []byte(unit)); e != nil {
		return e
	}
	if e = systemctl("daemon-reload"); e != nil {
		return e
	}
	return systemctl("enable", "--now", "pocketrelay.service")
}
func lockDir(dir string) (*os.File, error) {
	f, e := os.OpenFile(filepath.Join(dir, "daemon.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	if e = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		f.Close()
		return nil, errors.New("stop the daemon before this operation")
	}
	return f, nil
}
func transfer(dir string, c Config, command, out, from string) error {
	lock, e := lockDir(dir)
	if e != nil {
		return e
	}
	defer lock.Close()
	source, dest := dir, out
	if command != "backup" {
		source, dest = from, dir
	}
	if source == "" || dest == "" {
		return errors.New("--out or --from directory required")
	}
	source, e = filepath.Abs(source)
	if e != nil {
		return e
	}
	dest, e = filepath.Abs(dest)
	if e != nil {
		return e
	}
	if source == dest || strings.HasPrefix(dest, source+string(os.PathSeparator)) {
		return errors.New("destination cannot be inside source")
	}
	if _, e = os.Stat(filepath.Join(source, "events.sqlite3")); e != nil {
		return e
	}
	if command == "backup" {
		if _, e = os.Stat(dest); !os.IsNotExist(e) {
			return errors.New("backup destination must not exist")
		}
		if e = os.MkdirAll(dest, 0700); e != nil {
			return e
		}
	} else {
		entries, e := os.ReadDir(dest)
		if e != nil {
			return e
		}
		for _, x := range entries {
			if x.Name() != "config.json" && x.Name() != "daemon.lock" {
				return errors.New("restore destination must be empty (config allowed)")
			}
		}
	}
	// VACUUM INTO creates a consistent SQLite snapshot, including committed WAL data.
	db, e := sql.Open("sqlite", filepath.Join(source, "events.sqlite3"))
	if e != nil {
		return e
	}
	defer db.Close()
	db.Exec("PRAGMA busy_timeout=10000")
	target := filepath.Join(dest, "events.sqlite3")
	_, e = db.Exec("VACUUM INTO ?", target)
	if e != nil {
		return e
	}
	entries, e := os.ReadDir(source)
	if e != nil {
		return e
	}
	for _, x := range entries {
		n := x.Name()
		if x.IsDir() || strings.HasSuffix(n, ".playback.wav") {
			continue
		}
		ext := filepath.Ext(n)
		if (ext == ".audio" || ext == ".blob" || ext == ".image") && validID.MatchString(strings.TrimSuffix(n, ext)) {
			if e = copyFile(filepath.Join(source, n), filepath.Join(dest, n)); e != nil {
				return e
			}
		}
	}
	// Credentials deliberately stay local; pair the phone after migration/restore.
	output(map[string]any{"completed": command, "destination": dest, "credentialsCopied": false, "note": "Use pair to configure the phone; backup contains private messages and recordings."})
	return nil
}
func (a *App) tunnel(ctx context.Context) {
	os.Remove(filepath.Join(a.dir, "tunnel-url"))
	pattern := regexp.MustCompile(`https://[a-z0-9-]+\.trycloudflare\.com`)
	for ctx.Err() == nil {
		protocol := a.cfg.TunnelProtocol
		if protocol == "" {
			protocol = "auto"
		}
		cmd := exec.CommandContext(ctx, a.cfg.Cloudflared, "tunnel", "--no-autoupdate", "--protocol", protocol, "--url", fmt.Sprintf("http://127.0.0.1:%d", a.cfg.IngestPort))
		pipe, e := cmd.StderrPipe()
		if e == nil {
			e = cmd.Start()
		}
		if e != nil {
			fmt.Println("Quick tunnel unavailable; install cloudflared or configure a stable HTTPS URL")
			return
		}
		scan := bufio.NewScanner(pipe)
		for scan.Scan() {
			if url := pattern.FindString(scan.Text()); url != "" {
				atomicWrite(filepath.Join(a.dir, "tunnel-url"), []byte(url))
				fmt.Println("Phone HTTPS tunnel ready; run pocketrelay --link")
			}
		}
		cmd.Wait()
		os.Remove(filepath.Join(a.dir, "tunnel-url"))
		select {
		case <-ctx.Done():
			return
		case <-time.After(10 * time.Second):
		}
	}
}
