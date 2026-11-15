package main

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseSubscriptionJSON(t *testing.T) {
	payload := []byte(`[
		{
			"remarks": "Germany PRO",
			"outbounds": [
				{
					"tag": "direct",
					"protocol": "freedom"
				},
				{
					"tag": "proxy-2",
					"protocol": "vless",
					"settings": {
						"vnext": [{
							"address": "de1.example.com",
							"port": 443,
							"users": [{
								"id": "11111111-2222-3333-4444-555555555555",
								"flow": "xtls-rprx-vision",
								"encryption": "none"
							}]
						}]
					},
					"streamSettings": {
						"network": "tcp",
						"security": "reality",
						"realitySettings": {
							"serverName": "cdn.example.com",
							"fingerprint": "chrome",
							"publicKey": "pub_key_123",
							"shortId": "abcd"
						}
					}
				},
				{
					"tag": "proxy",
					"protocol": "vless",
					"settings": {
						"vnext": [{
							"address": "xh.example.com",
							"port": 8443,
							"users": [{
								"id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
							}]
						}]
					},
					"streamSettings": {
						"network": "xhttp",
						"security": "tls",
						"tlsSettings": {
							"serverName": "xh.example.com",
							"fingerprint": "firefox"
						},
						"xhttpSettings": {
							"path": "/split",
							"host": "edge.example.com"
						}
					}
				}
			]
		}
	]`)

	got, err := parseSubscription(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{
		"vless://11111111-2222-3333-4444-555555555555@de1.example.com:443?type=tcp&security=reality&encryption=none&flow=xtls-rprx-vision&sni=cdn.example.com&fp=chrome&pbk=pub_key_123&sid=abcd#Germany%20PRO%202",
		"vless://aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee@xh.example.com:8443?type=xhttp&security=tls&encryption=none&sni=xh.example.com&fp=firefox&path=%2Fsplit&host=edge.example.com#Germany%20PRO",
	}

	if len(got) != len(want) {
		t.Fatalf("got %d links, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("link[%d]:\n got: %s\nwant: %s", i, got[i], want[i])
		}
	}
}

func TestParseSubscriptionBase64(t *testing.T) {
	raw := "vless://111@one.example.com:443?security=reality#One\nvmess://ignored\nvless://222@two.example.com:443?type=ws#Two\n"
	encoded := base64.StdEncoding.EncodeToString([]byte(raw))

	got, err := parseSubscription([]byte(encoded))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{
		"vless://111@one.example.com:443?security=reality#One",
		"vless://222@two.example.com:443?type=ws#Two",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d links, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("link[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestDeviceHWID(t *testing.T) {
	hwid := deviceHWID()
	if len(hwid) != 16 {
		t.Fatalf("len(deviceHWID()) = %d, want 16 (%q)", len(hwid), hwid)
	}
	for _, c := range hwid {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Fatalf("non-hex char %q in %q", c, hwid)
		}
	}
}

func TestFetchAndParseFallback(t *testing.T) {
	var seenAgents []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua := r.Header.Get("User-Agent")
		seenAgents = append(seenAgents, ua)
		if strings.HasPrefix(ua, "Happ/") {
			if r.Header.Get("X-Hwid") == "" {
				t.Errorf("expected X-Hwid header on Happ request")
			}
			_, _ = w.Write([]byte("[]"))
			return
		}
		encoded := base64.StdEncoding.EncodeToString([]byte("vless://uuid@host:443?security=tls#Fallback\n"))
		_, _ = w.Write([]byte(encoded))
	}))
	defer srv.Close()

	links, err := fetchAndParse(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(links) != 1 || links[0] != "vless://uuid@host:443?security=tls#Fallback" {
		t.Fatalf("unexpected links: %v", links)
	}
	if len(seenAgents) != 2 || !strings.HasPrefix(seenAgents[0], "Happ/") || !strings.HasPrefix(seenAgents[1], "v2rayNG/") {
		t.Fatalf("unexpected User-Agent sequence: %v", seenAgents)
	}
}

func TestFormatLinks(t *testing.T) {
	links := []string{
		"vless://111@a.example:443?type=tcp&security=reality#Germany%20PRO",
		"vless://222@b.example:443?type=ws#Auto",
	}

	txtOut, err := formatLinks(links, "txt")
	if err != nil {
		t.Fatalf("txt format error: %v", err)
	}
	wantTXT := links[0] + "\n" + links[1] + "\n"
	if txtOut != wantTXT {
		t.Errorf("txt mismatch:\n got: %q\nwant: %q", txtOut, wantTXT)
	}

	b64Out, err := formatLinks(links, "base64")
	if err != nil {
		t.Fatalf("base64 format error: %v", err)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64Out))
	if err != nil {
		t.Fatalf("decode base64 output: %v", err)
	}
	if string(decoded) != strings.Join(links, "\n") {
		t.Errorf("base64 decoded mismatch: %q", string(decoded))
	}

	jsonOut, err := formatLinks(links, "json")
	if err != nil {
		t.Fatalf("json format error: %v", err)
	}
	if strings.Contains(jsonOut, `\u0026`) {
		t.Errorf("expected unescaped '&' in JSON output, got: %s", jsonOut)
	}
	var parsed []exportedServer
	if err := json.Unmarshal([]byte(jsonOut), &parsed); err != nil {
		t.Fatalf("unmarshal json output: %v", err)
	}
	if len(parsed) != 2 || parsed[0].Name != "Germany PRO" || parsed[0].Link != links[0] {
		t.Errorf("unexpected parsed json: %+v", parsed)
	}

	if _, err := formatLinks(links, "xml"); err == nil {
		t.Errorf("expected error for unsupported format")
	}
}
