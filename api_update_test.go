package spaceship

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/libdns/libdns"
)

func TestUpdateUsesRelativeNamesAndDeletesOldAddress(t *testing.T) {
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method)
		if r.URL.Path != "/v1/dns/records/example.com" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		switch r.Method {
		case http.MethodGet:
			fmt.Fprint(w, `{"items":[{"type":"AAAA","name":"@","address":"2001:db8::1","ttl":300}],"total":1}`)
		case http.MethodPut:
			var p struct {
				Items []struct {
					Name    string `json:"name"`
					Address string `json:"address"`
				} `json:"items"`
			}
			if err := json.NewDecoder(r.Body).Decode(&p); err != nil || len(p.Items) != 1 || p.Items[0].Name != "@" || p.Items[0].Address != "2001:db8::2" {
				t.Errorf("invalid put: %+v, %v", p, err)
			}
			w.WriteHeader(204)
		case http.MethodDelete:
			var p []struct {
				Name    string `json:"name"`
				Address string `json:"address"`
				Type    string `json:"type"`
			}
			if err := json.NewDecoder(r.Body).Decode(&p); err != nil || len(p) != 1 || p[0].Name != "@" || p[0].Address != "2001:db8::1" || p[0].Type != "AAAA" {
				t.Errorf("invalid delete: %+v, %v", p, err)
			}
			w.WriteHeader(204)
		default:
			t.Errorf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()
	p := newProvider()
	p.APIKey = "key"
	p.APISecret = "secret"
	p.BaseURL = server.URL
	if err := p.Provision(caddy.Context{}); err != nil {
		t.Fatal(err)
	}
	rec := libdns.RR{Name: "example.com.", Type: "AAAA", Data: "2001:db8::2", TTL: time.Minute}
	if _, err := p.SetRecords(context.Background(), "example.com.", []libdns.Record{rec}); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(calls) != "[GET PUT DELETE]" {
		t.Fatalf("unexpected calls: %v", calls)
	}
}
