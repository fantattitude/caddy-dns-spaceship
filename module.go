package spaceship

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	libdnsspaceship "github.com/empressmarissa/libdns_spaceship"
)

// Provider lets Caddy read and manipulate DNS records hosted by the Spaceship
// DNS provider. It adapts the libdns Spaceship provider for use in Caddy.
type Provider struct {
	*libdnsspaceship.Provider

	// APITimeout is the HTTP timeout configured by Caddy. The maintained
	// libdns provider deliberately does not serialize its HTTPClient, so this
	// adapter-owned field keeps the setting in JSON configurations.
	APITimeout caddy.Duration `json:"api_timeout,omitempty"`
}

func init() {
	caddy.RegisterModule(Provider{})
}

// CaddyModule returns the Caddy module information.
func (Provider) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID: "dns.providers.spaceship",
		New: func() caddy.Module {
			return &Provider{Provider: new(libdnsspaceship.Provider)}
		},
	}
}

func (p *Provider) ensureProvider() error {
	if p == nil {
		return fmt.Errorf("spaceship: provider is nil")
	}
	if p.Provider == nil {
		p.Provider = new(libdnsspaceship.Provider)
	}
	return nil
}

// Provision sets up the module. Implements caddy.Provisioner.
func (p *Provider) Provision(ctx caddy.Context) error {
	if err := p.ensureProvider(); err != nil {
		return err
	}

	// Expand placeholders before environment fallback so explicit Caddy
	// configuration wins, while an unresolved/empty field can still use the
	// maintained provider's environment handling.
	repl := caddy.NewReplacer()
	p.APIKey = repl.ReplaceAll(p.APIKey, "")
	p.APISecret = repl.ReplaceAll(p.APISecret, "")
	p.BaseURL = repl.ReplaceAll(p.BaseURL, "")
	p.Provider.PopulateFromEnv()

	// Environment values may themselves contain placeholders. This is cheap,
	// and keeps the behavior of the original adapter.
	p.APIKey = repl.ReplaceAll(p.APIKey, "")
	p.APISecret = repl.ReplaceAll(p.APISecret, "")
	p.BaseURL = repl.ReplaceAll(p.BaseURL, "")

	if p.PageSize < 0 {
		return fmt.Errorf("spaceship: page_size must not be negative")
	}
	if p.APITimeout < 0 {
		return fmt.Errorf("spaceship: api_timeout must not be negative")
	}

	// An explicit adapter timeout takes precedence over a client timeout. If a
	// caller supplied a client, preserve it and only repair a zero timeout;
	// otherwise leave a nil client alone so libdns can apply its 30s default.
	if p.APITimeout > 0 {
		if p.HTTPClient == nil {
			p.HTTPClient = &http.Client{}
		}
		p.HTTPClient.Timeout = time.Duration(p.APITimeout)
	} else if p.HTTPClient != nil && p.HTTPClient.Timeout == 0 {
		p.HTTPClient.Timeout = 30 * time.Second
	}

	if p.APIKey == "" || p.APISecret == "" {
		return fmt.Errorf("spaceship: api_key and api_secret are required")
	}
	return nil
}

// UnmarshalCaddyfile sets up the DNS provider from Caddyfile tokens. The
// optional inline form is `spaceship <api_key> <api_secret>`; block options
// are documented in README.md.
func (p *Provider) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	if err := p.ensureProvider(); err != nil {
		return err
	}

	seen := make(map[string]bool)
	for d.Next() {
		// Optional inline args: api_key api_secret.
		if d.NextArg() {
			if p.APIKey != "" || seen["api_key"] {
				return d.Err("api_key already set")
			}
			p.APIKey = d.Val()
			seen["api_key"] = true
		}
		if d.NextArg() {
			if p.APISecret != "" || seen["api_secret"] {
				return d.Err("api_secret already set")
			}
			p.APISecret = d.Val()
			seen["api_secret"] = true
		}
		if d.NextArg() {
			return d.ArgErr()
		}

		for nesting := d.Nesting(); d.NextBlock(nesting); {
			name := d.Val()
			canonical := map[string]string{
				"api_key":      "api_key",
				"api_secret":   "api_secret",
				"apisecret":    "api_secret",
				"api_url":      "base_url",
				"api_baseurl":  "base_url",
				"base_url":     "base_url",
				"api_pagesize": "page_size",
				"pagesize":     "page_size",
				"page_size":    "page_size",
				"api_timeout":  "api_timeout",
				"timeout":      "api_timeout",
			}[name]
			if canonical == "" {
				return d.Errf("unrecognized subdirective '%s'", name)
			}
			if seen[canonical] {
				return d.Errf("%s already set", canonical)
			}
			seen[canonical] = true

			switch canonical {
			case "api_key", "api_secret", "base_url":
				if !d.NextArg() {
					return d.Errf("%s requires value", name)
				}
				value := d.Val()
				if d.NextArg() {
					return d.ArgErr()
				}
				switch canonical {
				case "api_key":
					if p.APIKey != "" {
						return d.Err("api_key already set")
					}
					p.APIKey = value
				case "api_secret":
					if p.APISecret != "" {
						return d.Err("api_secret already set")
					}
					p.APISecret = value
				case "base_url":
					if p.BaseURL != "" {
						return d.Err("base_url already set")
					}
					p.BaseURL = value
				}
			case "page_size":
				if !d.NextArg() {
					return d.Errf("%s requires value", name)
				}
				pageSize, err := strconv.Atoi(d.Val())
				if err != nil || pageSize <= 0 {
					return d.Errf("%s must be a positive integer", name)
				}
				if d.NextArg() {
					return d.ArgErr()
				}
				if p.PageSize != 0 {
					return d.Err("page_size already set")
				}
				p.PageSize = pageSize
			case "api_timeout":
				if !d.NextArg() {
					return d.Errf("%s requires value (seconds)", name)
				}
				seconds, err := strconv.Atoi(d.Val())
				if err != nil || seconds <= 0 {
					return d.Errf("%s must be a positive integer (seconds)", name)
				}
				if d.NextArg() {
					return d.ArgErr()
				}
				if p.APITimeout != 0 {
					return d.Err("api_timeout already set")
				}
				p.APITimeout = caddy.Duration(time.Duration(seconds) * time.Second)
			}
		}
	}
	if p.APIKey == "" || p.APISecret == "" {
		return d.Err("missing api_key or api_secret")
	}
	return nil
}

// Interface guards.
var (
	_ caddy.Module          = (*Provider)(nil)
	_ caddy.Provisioner     = (*Provider)(nil)
	_ caddyfile.Unmarshaler = (*Provider)(nil)
)
