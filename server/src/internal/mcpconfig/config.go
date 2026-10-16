// Package mcpconfig contains the non-secret, owner-controlled MCP configuration.
package mcpconfig

import (
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const Scope = "records:read"

type Config struct {
	Enabled      bool   `json:"enabled"`
	PublicURL    string `json:"publicUrl"`
	Issuer       string `json:"issuer"`
	OwnerSubject string `json:"ownerSubject"`
}

func (c Config) Validate() error {
	if !c.Enabled {
		return nil
	}
	for _, raw := range []string{c.PublicURL, c.Issuer} {
		u, e := url.Parse(raw)
		if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.ContainsAny(raw, "\r\n\t ") {
			return errors.New("publicUrl and issuer must be HTTPS URLs without credentials or query")
		}
	}
	u, _ := url.Parse(c.PublicURL)
	if u.Path != "" && u.Path != "/" {
		return errors.New("publicUrl must be a base URL without /mcp")
	}
	if strings.TrimSpace(c.OwnerSubject) == "" || len(c.OwnerSubject) > 512 {
		return errors.New("ownerSubject is required; only this account may read records")
	}
	return nil
}
func (c Config) Resource() string { return strings.TrimRight(c.PublicURL, "/") + "/mcp" }
func Load(dir string) (Config, error) {
	var c Config
	b, e := os.ReadFile(filepath.Join(dir, "mcp.json"))
	if e != nil {
		return c, e
	}
	if len(b) > 16384 {
		return c, errors.New("MCP config too large")
	}
	e = json.Unmarshal(b, &c)
	if e != nil {
		return c, e
	}
	return c, c.Validate()
}
func Save(dir string, c Config) error {
	if e := c.Validate(); e != nil {
		return e
	}
	b, e := json.MarshalIndent(c, "", "  ")
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(dir, ".mcp-config-*")
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
	closeErr := f.Close()
	if e != nil {
		return e
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), filepath.Join(dir, "mcp.json"))
}
