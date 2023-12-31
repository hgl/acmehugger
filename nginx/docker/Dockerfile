# syntax=docker/dockerfile:1
FROM golang:1.21-bookworm as build

WORKDIR /go/src/acmehugger

RUN --mount=target=. \
	--mount=type=cache,target=/root/.cache/go-build \
	--mount=type=cache,target=/go/pkg/mod \
	<<EOF bash --posix
		set -euo pipefail
		ver=$(git describe --tags)
		ver=\${ver#v}
		go install \
			-trimpath \
			-ldflags "
				-s -w
				-X 'github.com/hgl/acmehugger/acme.CertsDir=/var/lib/acmehugger/acme/live'
				-X 'github.com/hgl/acmehugger/acme.ChallengeDir=/var/lib/acmehugger/acme-challenge'
				-X 'github.com/hgl/acmehugger.Version=\$ver'
			" \
			./nginx/nginxh
EOF

FROM debian:bookworm-slim

ARG NGINX_VERSION=1.25.1
ARG NGINX_CHECKSUM=f09071ac46e0ea3adc0008ef0baca229fc6b4be4533baef9bbbfba7de29a8602
ARG BROTLI_VERSION=6e975bcb015f62e1f303054897783355e2a877dc

RUN <<EOF bash --posix
	set -euo pipefail
	export DEBIAN_FRONTEND=noninteractive
	apt-get update
	apt-get install -y --no-install-recommends \
		curl ca-certificates git gcc make libpcre2-dev libssl-dev libssl3 \
		zlib1g-dev zlib1g pkg-config

	mkdir -p \
		/nginx/src \
		/nginx/brotli
	curl -fLo /nginx/src.tar.gz https://nginx.org/download/nginx-${NGINX_VERSION}.tar.gz
	sha256sum -c - <<-EOF2
		$NGINX_CHECKSUM /nginx/src.tar.gz
	EOF2

	cd /nginx/brotli
	git init
	git remote add origin https://github.com/google/ngx_brotli.git
	git fetch --depth 1 origin $BROTLI_VERSION
	git checkout FETCH_HEAD
	git submodule update --init --recursive --depth 1

	cd /nginx/src
	tar -xzf ../src.tar.gz --strip-component 1
	./configure \
		--prefix=/usr/share/nginx \
		--sbin-path=/usr/sbin/nginx \
		--modules-path=/usr/lib/nginx/modules \
		--conf-path=/etc/nginx/nginx.conf \
		--error-log-path=stderr \
		--http-log-path=/var/log/nginx/access.log \
		--pid-path=/var/run/nginx.pid \
		--lock-path=/var/lock/nginx.lock \
		--http-client-body-temp-path=/var/lib/nginx/body \
		--http-proxy-temp-path=/var/lib/nginx/proxy \
		--http-fastcgi-temp-path=/var/lib/nginx/fastcgi \
		--http-uwsgi-temp-path=/var/lib/nginx/uswgi \
		--http-scgi-temp-path=/var/lib/nginx/scgi \
		--with-pcre-jit \
		--with-http_ssl_module \
		--with-http_v2_module \
		--with-http_v3_module \
		--with-stream \
		--with-stream_ssl_module \
		--with-stream_ssl_preread_module \
		--add-module=../brotli
	nproc=$(nproc)
	make -j $((nproc + 1))
	make install

	mkdir -p \
		/var/lib/nginx \
		/var/log/nginx \
		/var/lib/acmehugger/nginx/conf \
		/var/lib/acmehugger/acme/accounts \
		/var/lib/acmehugger/acme/live \
		/var/lib/acmehugger/acme-challenge

	ln -sf /dev/stdout /var/log/nginx/access.log
	ln -sf /dev/stderr /var/log/nginx/error.log

	apt-get purge -y --autoremove \
		curl git gcc make libpcre2-dev libssl-dev zlib1g-dev \
		pkg-config
	apt-get clean
	rm -rf \
		/nginx \
		/var/lib/apt/lists/* \
		/var/log/alternatives.log \
		/var/log/dpkg.log \
		/var/log/apt/*
EOF

COPY --from=build /go/bin/nginxh /usr/sbin/

VOLUME [ "/var/lib/acmehugger/acme" ]

ENTRYPOINT [ "nginxh" ]
CMD [ "-g", "daemon off;" ]
