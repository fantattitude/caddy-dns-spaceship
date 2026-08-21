# Spaceship DNS provider module for Caddy

[![CI](https://github.com/empressmarissa/caddy-dns-spaceship/actions/workflows/ci.yml/badge.svg)](https://github.com/empressmarissa/caddy-dns-spaceship/actions/workflows/ci.yml)

This maintained fork provides a [Caddy](https://caddyserver.com/) DNS provider
for the [Spaceship DNS API](https://www.spaceship.com/). It adapts
[`github.com/empressmarissa/libdns_spaceship`](https://github.com/empressmarissa/libdns_spaceship)
v1.2.0 and keeps the Caddy module ID and provider name used by the upstream
adapter.

## Install

Build Caddy with [xcaddy](https://github.com/caddyserver/xcaddy), using the
released adapter module:

```sh
xcaddy build --with github.com/empressmarissa/caddy-dns-spaceship@v1.2.0
```

The adapter requires Caddy v2.10.2 or newer. The dependency chain is direct:
Caddy loads this module, which uses
`github.com/empressmarissa/libdns_spaceship` v1.2.0. No `replace` directive or
local replacement is needed.

## Caddy module name

```text
dns.providers.spaceship
```

In Caddy JSON, the DNS challenge provider name is `spaceship`.

## JSON configuration

Configure the ACME issuer's DNS challenge provider with the maintained
libdns field names:

```json
{
	"module": "acme",
	"challenges": {
		"dns": {
			"provider": {
				"name": "spaceship",
				"api_key": "YOUR_SPACESHIP_API_KEY",
				"api_secret": "YOUR_SPACESHIP_API_SECRET",
				"base_url": "https://spaceship.dev/api",
				"page_size": 100,
				"api_timeout": "30s"
			}
		}
	}
}
```

`base_url` and `page_size` are optional. `api_timeout` accepts Caddy duration
strings such as `"30s"`, `"1m"`, or `"500ms"`; as with Caddy's
`caddy.Duration`, a JSON number is interpreted as nanoseconds. The effective
default is 30 seconds. A caller-provided HTTP client and its nonzero timeout
are preserved.

## Caddyfile configuration

The inline form takes an API key followed by an API secret:

```caddyfile
tls {
	dns spaceship YOUR_API_KEY YOUR_API_SECRET
}
```

The block form supports these options:

```caddyfile
tls {
	dns spaceship {
		api_key    {env.LIBDNS_SPACESHIP_APIKEY}
		api_secret {env.LIBDNS_SPACESHIP_APISECRET}
		api_url    https://spaceship.dev/api
		api_pagesize 200
		api_timeout 20
	}
}
```

`api_timeout` uses a positive integer number of seconds in the Caddyfile. The
following aliases are accepted for compatibility:

| Canonical option | Aliases |
| --- | --- |
| `api_secret` | `apisecret` |
| `api_url` | `api_baseurl`, `base_url` |
| `api_pagesize` | `pagesize`, `page_size` |
| `api_timeout` | `timeout` |

Each option may be specified only once, including across its aliases.

## Environment and placeholders

Use Caddy placeholders such as `{env.LIBDNS_SPACESHIP_APIKEY}` in Caddyfile
values. When a value is unset, the maintained libdns provider fills it from
these environment variables:

| Field | Environment variable |
| --- | --- |
| API key | `LIBDNS_SPACESHIP_APIKEY` |
| API secret | `LIBDNS_SPACESHIP_APISECRET` |
| Base URL | `LIBDNS_SPACESHIP_BASEURL` |
| Page size | `LIBDNS_SPACESHIP_PAGESIZE` |
| HTTP timeout (seconds) | `LIBDNS_SPACESHIP_TIMEOUT` |

API key and API secret are required after placeholder expansion and environment
fallback. Do not commit credentials to configuration or source control.

## Migration from the upstream adapter

The module ID (`dns.providers.spaceship`), provider name (`spaceship`), and
Caddyfile forms remain compatible with `caddy-dns/spaceship`. Change the
`xcaddy` module path to this maintained fork and use the JSON keys `api_key`,
`api_secret`, `base_url`, `page_size`, and `api_timeout`. The fork uses the
released maintained libdns module directly, so old replacement directives
should be removed.

## License

MIT License (see [LICENSE](./LICENSE)). This project retains attribution to the
upstream Caddy DNS provider adapter.
