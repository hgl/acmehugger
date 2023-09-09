# ACME Hugger Install

## Binaries

Download the binary from the [releases page](https://github.com/hgl/acmehugger/releases).

The binary assumes that a few directories and files exits. You need to create them before running the binary:

- `/var/lib/acmehugger/acme/accounts/` for ACME accounts and certificates.
- `/etc/ssl/acme/` for symlinks to certificates in the previous directory.
- `/var/lib/acmehugger/acme/challenge/` for ACME challenge answers.
- `/usr/share/acmehugger/hook.d/` for [ACME hooks](https://github.com/hgl/acmehugger/blob/main/docs/Reference.md#hooks).
- `/etc/nginx/` for original Nginx configuration files.
- `/etc/nginx/nginx.conf` for original Nginx configuration entrypoint.
- `/var/lib/acmehugger/nginx/conf/` for generated Nginx configuration files.

## Source

```
$ go install github.com/hgl/acmehugger/nginx/nginxh@latest
```

ACME Hugger assumes that a few directories exits. You can define their paths with the `-ldflag` flag:

```
$ go install -ldflags "
    -X 'github.com/hgl/acmehugger/acme.AccountsDir=/path/to/dir'
    -X 'github.com/hgl/acmehugger/acme.CertsDir=/path/to/dir'
" github.com/hgl/acmehugger/nginx/nginxh@latest
```

A list of variables (and their default value) under `github.com/hgl/` available for redefining:

- `acmehugger.StateDir` (`/var/lib/acmehugger`)
- `acmehugger/acme.AccountsDir` (`${acmehugger.StateDir}/acme/accounts`)
- `acmehugger/acme.ChallengeDir` (`${acmehugger.StateDir}/acme/accounts`)
- `acmehugger/acme.CertsDir` (`/etc/ssl/acme`)
- `acmehugger/acme.HooksDir` (`/usr/share/acmehugger/hook.d`)
- `acmehugger/nginx.ConfDir` (`/etc/nginx`)
- `acmehugger/nginx.Conf` (`${acmehugger/nginx.ConfDir}/nginx.conf`)
- `acmehugger/nginx.ConfOutDir` (`${acmehugger.StateDir}/nginx/conf`)

You need to create them before running the binary. See [binaries install](#binaries) for the meaning of each directory and file.

## Docker

### Pull
```
$ docker pull hgl0/nginxh
```
or
```
$ docker pull ghcr.io/hgl/nginxh
```

### Run

```
$ docker run hgl0/nginxh -h
```

The Docker image comes with a mainline Nginx with brotli module added. It uses most default locations as noted in [Binaries](#binaries) with a few changes:

- Symlinks to certificates are stored in `/var/lib/acmehugger/acme/live/`
- ACME challenge answers are stored in `/var/lib/acmehugger/acme-challenge/`

This is done so that `/var/lib/acmehugger/acme` can be a volume and will persist ACME accounts, certificates and symlinks.
