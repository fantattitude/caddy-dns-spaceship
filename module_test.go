package spaceship

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	libdnsspaceship "github.com/empressmarissa/libdns_spaceship"
	"github.com/libdns/libdns"
)

func newProvider() *Provider {
	return &Provider{Provider: new(libdnsspaceship.Provider)}
}

func parseProvider(t *testing.T, input string) *Provider {
	t.Helper()
	tokens, err := caddyfile.Tokenize([]byte(input), "test.caddy")
	if err != nil {
		t.Fatalf("tokenize Caddyfile: %v", err)
	}
	p := newProvider()
	if err := p.UnmarshalCaddyfile(caddyfile.NewDispenser(tokens)); err != nil {
		t.Fatalf("unmarshal Caddyfile: %v", err)
	}
	return p
}

func parseProviderError(t *testing.T, input string) error {
	t.Helper()
	tokens, err := caddyfile.Tokenize([]byte(input), "test.caddy")
	if err != nil {
		return err
	}
	p := newProvider()
	return p.UnmarshalCaddyfile(caddyfile.NewDispenser(tokens))
}

func TestModuleRegistration(t *testing.T) {
	info, err := caddy.GetModule("dns.providers.spaceship")
	if err != nil {
		t.Fatalf("module is not registered: %v", err)
	}
	if info.ID != "dns.providers.spaceship" {
		t.Fatalf("unexpected module ID %q", info.ID)
	}
	p, ok := info.New().(*Provider)
	if !ok || p.Provider == nil {
		t.Fatalf("module constructor returned %#v", info.New())
	}
}

func TestUnmarshalCaddyfileInline(t *testing.T) {
	p := parseProvider(t, `spaceship key123 secret456`)
	if p.APIKey != "key123" || p.APISecret != "secret456" {
		t.Fatalf("unexpected credentials: %q / %q", p.APIKey, p.APISecret)
	}
}

func TestUnmarshalCaddyfileBlockAndAliases(t *testing.T) {
	p := parseProvider(t, `spaceship {
		api_key key123
		apisecret secret456
		base_url https://example.test/api
		page_size 250
		timeout 15
	}`)
	if p.APIKey != "key123" || p.APISecret != "secret456" {
		t.Fatalf("credentials mismatch")
	}
	if p.PageSize != 250 {
		t.Fatalf("expected page size 250, got %d", p.PageSize)
	}
	if p.APITimeout != caddy.Duration(15*time.Second) {
		t.Fatalf("expected timeout 15s, got %s", time.Duration(p.APITimeout))
	}
	if p.HTTPClient != nil {
		t.Fatalf("Caddyfile timeout should remain serializable, got HTTP client %#v", p.HTTPClient)
	}
	if p.BaseURL != "https://example.test/api" {
		t.Fatalf("unexpected base URL: %s", p.BaseURL)
	}
}

func TestUnmarshalCaddyfileAliasesAndDuplicates(t *testing.T) {
	aliases := []string{
		`api_secret secret`, `apisecret secret`,
		`api_url https://example.test`, `api_baseurl https://example.test`, `base_url https://example.test`,
		`api_pagesize 200`, `pagesize 200`, `page_size 200`,
		`api_timeout 20`, `timeout 20`,
	}
	for _, directive := range aliases {
		input := "spaceship {\napi_key key\n" + directive + "\n"
		if strings.HasPrefix(directive, "api_secret") || strings.HasPrefix(directive, "apisecret") {
			input += "}\n"
		} else {
			input += "api_secret secret\n}\n"
		}
		if err := parseProviderError(t, input); err != nil {
			t.Errorf("%q rejected: %v", directive, err)
		}
	}

	duplicates := []string{
		`api_key a
		api_key b`,
		`api_secret a
		apisecret b`,
		`api_url https://one
		base_url https://two`,
		`api_pagesize 100
		page_size 200`,
		`api_timeout 10
		timeout 20`,
	}
	for _, body := range duplicates {
		if err := parseProviderError(t, "spaceship {\napi_key key\napi_secret secret\n"+body+"\n}\n"); err == nil {
			t.Errorf("duplicate directives accepted: %q", body)
		}
	}
}

func TestUnmarshalCaddyfileRejectsBadArguments(t *testing.T) {
	cases := []string{
		`spaceship key secret extra`,
		"spaceship {\napi_key\napi_secret secret\n}\n",
		"spaceship {\napi_key key extra\napi_secret secret\n}\n",
		"spaceship {\napi_key key\napi_secret secret\napi_pagesize\n}\n",
		"spaceship {\napi_key key\napi_secret secret\napi_pagesize 0\n}\n",
		"spaceship {\napi_key key\napi_secret secret\napi_timeout 0\n}\n",
		"spaceship {\napi_key key\napi_secret secret\nunknown value\n}\n",
		"spaceship {\napi_key key\n}\n",
	}
	for _, input := range cases {
		if err := parseProviderError(t, input); err == nil {
			t.Errorf("invalid Caddyfile accepted: %q", input)
		}
	}
}

func TestJSONConfigAndTimeout(t *testing.T) {
	var p Provider
	err := json.Unmarshal([]byte(`{
		"api_key":"json-key",
		"api_secret":"json-secret",
		"base_url":"https://example.test/api",
		"page_size":250,
		"api_timeout":"15s"
	}`), &p)
	if err != nil {
		t.Fatalf("unmarshal JSON: %v", err)
	}
	if p.Provider == nil || p.APIKey != "json-key" || p.APISecret != "json-secret" || p.BaseURL != "https://example.test/api" || p.PageSize != 250 {
		t.Fatalf("unexpected JSON provider: %#v", p)
	}
	if p.APITimeout != caddy.Duration(15*time.Second) {
		t.Fatalf("unexpected JSON timeout: %s", time.Duration(p.APITimeout))
	}
	if err := p.Provision(caddy.Context{}); err != nil {
		t.Fatalf("provision JSON provider: %v", err)
	}
	if p.HTTPClient == nil || p.HTTPClient.Timeout != 15*time.Second {
		t.Fatalf("JSON timeout was not applied: %#v", p.HTTPClient)
	}

	b, err := json.Marshal(&p)
	if err != nil {
		t.Fatalf("marshal JSON provider: %v", err)
	}
	if !strings.Contains(string(b), `"api_timeout":15000000000`) {
		t.Fatalf("timeout did not survive JSON serialization: %s", b)
	}
	var roundTrip Provider
	if err := json.Unmarshal(b, &roundTrip); err != nil {
		t.Fatalf("unmarshal serialized provider: %v", err)
	}
	if roundTrip.APITimeout != p.APITimeout {
		t.Fatalf("timeout changed during JSON round trip: %s", time.Duration(roundTrip.APITimeout))
	}
}

func TestProvisionPlaceholdersEnvironmentAndPrecedence(t *testing.T) {
	t.Setenv("LIBDNS_SPACESHIP_APIKEY", "env-key")
	t.Setenv("LIBDNS_SPACESHIP_APISECRET", "env-secret")
	t.Setenv("LIBDNS_SPACESHIP_BASEURL", "https://env.example/api")

	p := newProvider()
	p.APIKey = "{env.LIBDNS_SPACESHIP_APIKEY}"
	p.APISecret = "{env.LIBDNS_SPACESHIP_APISECRET}"
	if err := p.Provision(caddy.Context{}); err != nil {
		t.Fatalf("provision placeholders: %v", err)
	}
	if p.APIKey != "env-key" || p.APISecret != "env-secret" || p.BaseURL != "https://env.example/api" {
		t.Fatalf("unexpected placeholder/env values: %#v", p.Provider)
	}

	p = newProvider()
	p.APIKey = "explicit-key"
	p.APISecret = "explicit-secret"
	p.BaseURL = "https://explicit.example/api"
	if err := p.Provision(caddy.Context{}); err != nil {
		t.Fatalf("provision explicit values: %v", err)
	}
	if p.APIKey != "explicit-key" || p.APISecret != "explicit-secret" || p.BaseURL != "https://explicit.example/api" {
		t.Fatalf("environment overrode explicit values: %#v", p.Provider)
	}
}

func TestProvisionTimeoutAndNilSafety(t *testing.T) {
	t.Setenv("LIBDNS_SPACESHIP_APIKEY", "key")
	t.Setenv("LIBDNS_SPACESHIP_APISECRET", "secret")

	p := &Provider{}
	if err := p.Provision(caddy.Context{}); err != nil {
		t.Fatalf("nil embedded provider should be provisionable: %v", err)
	}
	if p.Provider == nil {
		t.Fatal("embedded provider was not initialized")
	}
	if p.HTTPClient != nil {
		t.Fatalf("nil HTTP client should remain nil for libdns default: %#v", p.HTTPClient)
	}

	client := &http.Client{Timeout: 7 * time.Second}
	p = newProvider()
	p.APIKey, p.APISecret, p.HTTPClient = "key", "secret", client
	if err := p.Provision(caddy.Context{}); err != nil {
		t.Fatalf("provision custom timeout client: %v", err)
	}
	if p.HTTPClient != client || client.Timeout != 7*time.Second {
		t.Fatalf("nonzero custom timeout was changed: %#v", client)
	}

	client = &http.Client{}
	p = newProvider()
	p.APIKey, p.APISecret, p.HTTPClient = "key", "secret", client
	if err := p.Provision(caddy.Context{}); err != nil {
		t.Fatalf("provision zero timeout client: %v", err)
	}
	if client.Timeout != 30*time.Second {
		t.Fatalf("zero timeout client was not made safe: %s", client.Timeout)
	}

	p = newProvider()
	p.APIKey, p.APISecret, p.APITimeout = "key", "secret", caddy.Duration(12*time.Second)
	if err := p.Provision(caddy.Context{}); err != nil {
		t.Fatalf("provision explicit timeout: %v", err)
	}
	if p.HTTPClient == nil || p.HTTPClient.Timeout != 12*time.Second {
		t.Fatalf("explicit timeout was not applied: %#v", p.HTTPClient)
	}
}

func TestProvisionRejectsInvalidJSONValuesAndMissingCredentials(t *testing.T) {
	cases := []struct {
		name string
		p    *Provider
	}{
		{name: "page size", p: &Provider{Provider: &libdnsspaceship.Provider{APIKey: "key", APISecret: "secret", PageSize: -1}}},
		{name: "timeout", p: &Provider{Provider: &libdnsspaceship.Provider{APIKey: "key", APISecret: "secret"}, APITimeout: -1}},
		{name: "credentials", p: newProvider()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.p.Provision(caddy.Context{}); err == nil {
				t.Fatal("expected provision error")
			}
		})
	}
}

func TestAppendAndDeleteDelegateGenericTXT(t *testing.T) {
	type request struct {
		method string
		path   string
		body   string
	}
	requests := make(chan request, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requests <- request{method: r.Method, path: r.URL.Path, body: string(body)}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	p := newProvider()
	p.APIKey, p.APISecret, p.BaseURL = "key", "secret", server.URL
	if err := p.Provision(caddy.Context{}); err != nil {
		t.Fatalf("provision delegation provider: %v", err)
	}

	rr := libdns.RR{Name: "_acme-challenge", Type: "TXT", Data: "token", TTL: 30 * time.Second}
	added, err := p.AppendRecords(context.Background(), "example.com.", []libdns.Record{rr})
	if err != nil {
		t.Fatalf("append records: %v", err)
	}
	if len(added) != 1 || added[0].RR().Name != "_acme-challenge" || added[0].RR().Type != "TXT" || added[0].RR().Data != "token" {
		t.Fatalf("unexpected appended records: %#v", added)
	}
	appended := <-requests
	if appended.method != http.MethodPut || appended.path != "/v1/dns/records/example.com" || !strings.Contains(appended.body, `"name":"_acme-challenge"`) || !strings.Contains(appended.body, `"type":"TXT"`) || !strings.Contains(appended.body, `"value":"token"`) {
		t.Fatalf("unexpected append request: %#v", appended)
	}

	deleted, err := p.DeleteRecords(context.Background(), "example.com.", []libdns.Record{rr})
	if err != nil {
		t.Fatalf("delete records: %v", err)
	}
	if len(deleted) != 1 || deleted[0].RR().Name != rr.Name {
		t.Fatalf("unexpected deleted records: %#v", deleted)
	}
	removed := <-requests
	if removed.method != http.MethodDelete || removed.path != "/v1/dns/records/example.com" || !strings.Contains(removed.body, `"name":"_acme-challenge"`) {
		t.Fatalf("unexpected delete request: %#v", removed)
	}
	if strings.Contains(appended.body, "key") || strings.Contains(appended.body, "secret") || strings.Contains(removed.body, "key") || strings.Contains(removed.body, "secret") {
		t.Fatalf("credentials leaked into request body")
	}
}
