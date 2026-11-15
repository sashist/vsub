package main

import (
	"bufio"
	"bytes"
	"cmp"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type configItem struct {
	Remarks   string     `json:"remarks"`
	Outbounds []outbound `json:"outbounds"`
}

type outbound struct {
	Tag      string           `json:"tag"`
	Protocol string           `json:"protocol"`
	Settings outboundSettings `json:"settings"`
	Stream   streamSettings   `json:"streamSettings"`
}

type outboundSettings struct {
	Vnext []vnextServer `json:"vnext"`
}

type vnextServer struct {
	Address string      `json:"address"`
	Port    int         `json:"port"`
	Users   []vnextUser `json:"users"`
}

type vnextUser struct {
	ID         string `json:"id"`
	Flow       string `json:"flow"`
	Encryption string `json:"encryption"`
}

type streamSettings struct {
	Network  string          `json:"network"`
	Security string          `json:"security"`
	Reality  realitySettings `json:"realitySettings"`
	XHTTP    xhttpSettings   `json:"xhttpSettings"`
	WS       wsSettings      `json:"wsSettings"`
	TLS      tlsSettings     `json:"tlsSettings"`
}

type realitySettings struct {
	ServerName  string `json:"serverName"`
	Fingerprint string `json:"fingerprint"`
	PublicKey   string `json:"publicKey"`
	ShortID     string `json:"shortId"`
}

type xhttpSettings struct {
	Path string `json:"path"`
	Host string `json:"host"`
}

type wsSettings struct {
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers"`
}

type tlsSettings struct {
	ServerName  string `json:"serverName"`
	Fingerprint string `json:"fingerprint"`
}

func main() {
	if err := runInteractive(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

type exportedServer struct {
	Name string `json:"name"`
	Link string `json:"link"`
}

func formatLinks(links []string, format string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "txt":
		return strings.Join(links, "\n") + "\n", nil
	case "base64", "b64":
		raw := strings.Join(links, "\n")
		return base64.StdEncoding.EncodeToString([]byte(raw)) + "\n", nil
	case "json":
		servers := make([]exportedServer, 0, len(links))
		for _, link := range links {
			servers = append(servers, exportedServer{
				Name: linkName(link),
				Link: link,
			})
		}
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		if err := enc.Encode(servers); err != nil {
			return "", fmt.Errorf("encode json: %w", err)
		}
		return buf.String(), nil
	default:
		return "", fmt.Errorf("unsupported format %q (use txt, base64, json)", format)
	}
}

func loadLinks(rawURL string) ([]string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return nil, errors.New("url must start with http:// or https://")
	}
	return fetchAndParse(rawURL)
}

func expandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~"))
		}
	}
	return path
}

func deviceHWID() string {
	var seed string
	for _, p := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id"} {
		if b, err := os.ReadFile(p); err == nil {
			if s := strings.TrimSpace(string(b)); s != "" {
				seed = s
				break
			}
		}
	}
	if seed == "" {
		if host, err := os.Hostname(); err == nil && host != "" {
			seed = host
		}
	}
	if seed == "" {
		seed = "a1b2c3d4e5f6a7b8"
	}

	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:8])
}

func happHeaders(hwid string) map[string]string {
	return map[string]string{
		"User-Agent":      "Happ/3.13.0",
		"X-Device-Os":     "Android",
		"X-Device-Locale": "ru",
		"X-Device-Model":  "ELP-NX1",
		"X-Ver-Os":        "15",
		"X-Hwid":          hwid,
	}
}

func fetchAndParse(rawURL string) ([]string, error) {
	profiles := []map[string]string{
		happHeaders(deviceHWID()),
		{"User-Agent": "v2rayNG/1.8.23"},
	}

	var lastErr error
	for _, headers := range profiles {
		raw, err := fetchSubscription(rawURL, headers)
		if err != nil {
			lastErr = err
			continue
		}
		links, err := parseSubscription(raw)
		if err != nil {
			lastErr = err
			continue
		}
		if len(links) > 0 {
			return links, nil
		}
		lastErr = errors.New("no vless links found")
	}
	return nil, lastErr
}

var httpClient = &http.Client{Timeout: 15 * time.Second}

func fetchSubscription(rawURL string, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	// 16 MiB cap prevents unbounded reads on unexpected responses.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	return body, nil
}

func copyToClipboard(text string) error {
	type clipCmd struct {
		name string
		args []string
	}

	var candidates []clipCmd
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		candidates = append(candidates, clipCmd{"wl-copy", nil})
	}
	candidates = append(candidates,
		clipCmd{"xclip", []string{"-selection", "clipboard"}},
		clipCmd{"xsel", []string{"--clipboard", "--input"}},
		clipCmd{"pbcopy", nil},
	)

	for _, c := range candidates {
		path, err := exec.LookPath(c.name)
		if err != nil {
			continue
		}
		cmd := exec.Command(path, c.args...)
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s failed: %w", c.name, err)
		}
		return nil
	}
	return errors.New("clipboard tool not found (install wl-copy, xclip or xsel)")
}

func parseSubscription(data []byte) ([]string, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, errors.New("empty subscription")
	}

	if trimmed[0] == '[' || trimmed[0] == '{' {
		return parseJSONConfig(trimmed)
	}
	return parseTextOrBase64(trimmed)
}

func parseJSONConfig(data []byte) ([]string, error) {
	var items []configItem
	if data[0] == '{' {
		var item configItem
		if err := json.Unmarshal(data, &item); err != nil {
			return nil, fmt.Errorf("parse json: %w", err)
		}
		items = []configItem{item}
	} else {
		if err := json.Unmarshal(data, &items); err != nil {
			return nil, fmt.Errorf("parse json: %w", err)
		}
	}

	var links []string
	for _, item := range items {
		remarks := cmp.Or(item.Remarks, "Server")
		for _, out := range item.Outbounds {
			if out.Protocol != "vless" {
				continue
			}
			for _, server := range out.Settings.Vnext {
				if len(server.Users) == 0 || server.Address == "" || server.Port <= 0 {
					continue
				}
				user := server.Users[0]
				if user.ID == "" {
					continue
				}
				links = append(links, formatVLESSLink(remarks, out, server, user))
			}
		}
	}
	return links, nil
}

func formatVLESSLink(remarks string, out outbound, server vnextServer, user vnextUser) string {
	network := cmp.Or(out.Stream.Network, "tcp")
	security := cmp.Or(out.Stream.Security, "none")
	encryption := cmp.Or(user.Encryption, "none")

	sni := cmp.Or(out.Stream.Reality.ServerName, out.Stream.TLS.ServerName)
	fp := cmp.Or(out.Stream.Reality.Fingerprint, out.Stream.TLS.Fingerprint)

	var path, host string
	switch network {
	case "xhttp":
		path = out.Stream.XHTTP.Path
		host = out.Stream.XHTTP.Host
	case "ws":
		path = out.Stream.WS.Path
		host = cmp.Or(out.Stream.WS.Headers["Host"], out.Stream.WS.Headers["host"])
	}

	query := buildQuery(
		[2]string{"type", network},
		[2]string{"security", security},
		[2]string{"encryption", encryption},
		[2]string{"flow", user.Flow},
		[2]string{"sni", sni},
		[2]string{"fp", fp},
		[2]string{"pbk", out.Stream.Reality.PublicKey},
		[2]string{"sid", out.Stream.Reality.ShortID},
		[2]string{"path", path},
		[2]string{"host", host},
	)

	name := remarks
	if strings.Contains(out.Tag, "proxy-") {
		name = remarks + " " + strings.ReplaceAll(out.Tag, "proxy-", "")
	}

	addr := net.JoinHostPort(server.Address, strconv.Itoa(server.Port))
	return fmt.Sprintf("vless://%s@%s?%s#%s", user.ID, addr, query, url.PathEscape(name))
}

func parseTextOrBase64(data []byte) ([]string, error) {
	text := string(data)
	if !strings.Contains(text, "vless://") {
		decoded, ok := decodeBase64(data)
		if !ok {
			return nil, errors.New("unsupported subscription format")
		}
		text = string(decoded)
	}

	var links []string
	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "vless://") {
			links = append(links, line)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan links: %w", err)
	}
	return links, nil
}

func decodeBase64(data []byte) ([]byte, bool) {
	compact := strings.Map(func(r rune) rune {
		if r == '\r' || r == '\n' || r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, string(data))

	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	for _, enc := range encodings {
		if out, err := enc.DecodeString(compact); err == nil {
			return out, true
		}
	}
	return nil, false
}

func buildQuery(pairs ...[2]string) string {
	parts := make([]string, 0, len(pairs))
	for _, p := range pairs {
		if p[1] == "" {
			continue
		}
		parts = append(parts, url.QueryEscape(p[0])+"="+url.QueryEscape(p[1]))
	}
	return strings.Join(parts, "&")
}
