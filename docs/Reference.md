# ACME Hugger Reference

Currently, if HTTP01 challenge is used, all HTTP `server`s are added the `location /.well-known/acme-challenge/ { ... }` directive, to keep the logic simple. This might get optimized in the future.

After an ACME certificate is obtained, corresponding `ssl_certificate`, `ssl_certificate_key` and `ssl_trusted_certificate` directive are added to `server { ... }`.

ACME Hugger is designed to be idempotent, meaning you can restart it during the process, and it will continue issuing/renewing certificates or wait for the next renew time.

## CLI

`nginxh` passes all arguments to `nginx`, changing only the configuration file path. If `-h` is passes, it shows its own help instead of `nginx`'s.

By default, ACME hugger runs `nginx` to start Nginx', that name can be changed with the environment variable `NGINXBIN`. You can specify a path to avoid it searching in `$PATH`.

Setting the environment variable `ACMEHUGGER_DEBUG` to `1` enables more verbose logging.

## Scope

Directives in an inner block overrides those in the outer block:

```nginx
http {
    acme_server a.example.com;
    server {
        acme_server b.example.com;
    }
}
```

In this example `acme_server b.example.com` will be used for the server.

## Directives

### server_name
Default: server_name "";<br>
Context: server

Domains to add in the ACME certificate. Ignored if the domains are specified with `~` ( i.e., regular expression), in which case `acme_domain` should be used.

### acme_email email
Default: acme_email ""<br>
Context: main, http, server, acme

Email used for registration and recovery contact.

This directive is removed after read.

### acme_server url
Default: acme_server ""<br>
Context: main, http, server, acme

CA hostname (and optionally :port). If it's empty, `acme_staging` determines the default value.

This directive is removed after read.

### acme_staging on | off
Default: acme_staging off<br>
Context: main, http, server, acme

Ignored if `acme_server` is non-empty, otherwise, Let's Encrypt's production or staging URL is used, respectively.

This directive is removed after read.

#### acme_key ec256 | ec384 | rsa2048 | rsa3072 | rsa4096 | rsa8192
Default: acme_key ec256<br>
Context: main, http, server, acme

Key type to use for private keys.

This directive is removed after read.

### acme_challenge http | dns
Default: acme_challenge http<br>
Context: main, http, server, acme

ACME challenge to use.

This directive is removed after read.

### acme_days number
Default: acme_days 30<br>
Context: main, http, server, acme

The number of days left on a certificate to renew it.

This directive is removed after read.

### acme_dns name
Default: -<br>
Context: main, http, server, acme

DNS provider to use. Setting this also sets `acme_challenge` to `dns`. For a list of valid names, [see lego's document](https://go-acme.github.io/lego/dns/). Use each DNS provider's "code" value.

For example, for Amazon Route 53:

```
acme_dns route53;
```

This directive is removed after read.

### acme_dns_option key value

Options to use for the specified `acme_dns`. For a list of valid keys, [see lego's document](https://go-acme.github.io/lego/dns/).
They should be the lowercase of "Environment Variable Name" values.

For example, for Amazon Route 53:

```
acme_dns route53;
acme_dns_option aws_access_key_id xxxx;
```

This directive is removed after read.

### acme_defer directive
Default: -<br>
Context: server, acme

Directive after it is omitted from the configuration until the certificate exists.

### acme_domain domain ...
Default: -<br>
Context: server, acme

Domains to add in the ACME certificate. It has higher priority than `server_name`, and also supports wildcard.

This directive is removed after read.

### acme \{ ... }
Default: -<br>
Context: main

Allow obtaining certificates that aren't attached to any specific `server { ... }`.

This directive is removed after read.

## Hooks

Whenever a certificate is issued or renewed, ACME Hugger will call each executable in the hooks directory (`/usr/share/acmehugger/hook.d` by default) in turn (sorted by file name) with the following environment variables set:

| Names |
| --- |
| ACME_SERVER |
| ACME_EMAIL |
| ACME_DOMAIN |
