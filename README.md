# Spaceship DNS for Caddy

A fork of [empressmarissa/caddy-dns-spaceship](https://github.com/empressmarissa/caddy-dns-spaceship) that replaces an existing A or AAAA address when it changes. It returns an error if multiple records already exist for the same name and type; those need to be cleaned up separately.

Build Caddy with:

```sh
xcaddy build --with github.com/fantattitude/caddy-dns-spaceship
```

Configure the provider with your Spaceship API credentials:

```caddyfile
{
    acme_dns spaceship {env.SPACESHIP_API_KEY} {env.SPACESHIP_API_SECRET}
}
```

See the [original repository](https://github.com/empressmarissa/caddy-dns-spaceship) for configuration and usage details.
