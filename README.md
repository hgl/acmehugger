# ACME Hugger

Make load balancers like Nginx have native ACME capabilities.

- [How to Install](https://github.com/hgl/acmehugger/blob/main/docs/Install.md)
- [Reference](https://github.com/hgl/acmehugger/blob/main/docs/Reference.md)

## Why

Making a load balancer work with an ACME tool (e.g., [acme.sh](https://github.com/acmesh-official/acme.sh), [lego](https://go-acme.github.io/lego/))is quite involved and painful:

1. The load balancer cannot have HTTPS configuration before the certificates are obtained. So you need to edit the configuration at least once, and also manually reload the load balancer after that.
1. The load balancer needs to read ACME challenge answers from a location specified by the ACME tool.
1. Cron jobs have be set up to periodically renew the certificates and reload the load balancer after that.
1. With the above drawbacks, provisioning an HTTPS web server in an automatic way is quite challenging.

With ACME Hugger, you just add some ACME directives written in the load balancer's native configuration syntax, and let ACME Hugger handle all of the above for you, automatically.

## How

Given this Nginx configuration:

```nginx
# nginx.conf
http {
    acme_email acme@example.com;

    server {
        listen 80;
        acme_defer listen 443 ssl;
        server_name example.com;
    }
}
```

Start ACME Hugger for Nginx (notice the h in nginxh):

```
$ nginxh -c nginx.conf -g 'daemon off;'
```

ACME Hugger first changes this configuration into one with which Nginx is able to answer ACME HTTP01 challenges:

```nginx
http {
    server {
        listen 80;
        server_name example.com;
        location /.well-known/acme-challenge/ {
            root <acme challenge dir>;
        }
    }
}
```

It then runs Nginx, and talks to an ACME CA server (by default Let's Encrypt) to obtain the certificate for `example.com`, after which it updates the configuration to:

```nginx
http {
    server {
        listen 80;
        listen 443 ssl;
        server_name example.com;
        location /.well-known/acme-challenge/ {
            root <acme challenge dir>;
        }
        ssl_certificate <acme certs dir>/example.com.fullchain.crt;
        ssl_certificate_key <acme certs dir>/example.com.key;
        ssl_trusted_certificate <acme cert dir>/example.com.chain.crt;
    }
}
```
And reloads Nginx. It also waits for the right moment to automatically renew the certificates.

Notice that directives prefixed by `acme_defer` are hidden until certificates are obtained, at which point certificate related directives are appended. This is needed because Nginx considers it an error listening `ssl` without these directives.

## Todo

- HAProxy support
- Windows support

## Credit

ACME Hugger uses lego under the hood.
