.ONESHELL:
SHELL := bash
.SHELLFLAGS := --posix -euo pipefail -c
.DELETE_ON_ERROR:
.NOTINTERMEDIATE:

VER=$(patsubst v%,%,$(shell git describe --tags))

.PHONY: release

define BuildBin
  name := nginxh_$(VER)_$(1)_$(2)
  tardir := build/bin/$$(name)
  tar := $$(tardir).tar.gz

  release: $$(tar)
  $$(tar): tardir := $$(tardir)
  ifeq ($(1),windows)
    $$(tar): ext := .exe
  else
    $$(tar): ext :=
  endif
  $$(tar): | build/bin
	mkdir $$(tardir)
	trap 'rm -r $$(tardir)' EXIT
	GOOS=$(1) GOARCH=$(2) go build \
		-o $$(tardir)/nginxh$$(ext) \
		-trimpath \
		-ldflags " \
			-s -w -buildid= \
			-X 'github.com/hgl/acmehugger.Version=$(VER)' \
		" \
		./nginx/nginxh
	tar \
		-czC $$(dir $$(tardir)) \
		--owner 0 \
		--group 0 \
		-f $$@ \
		$$(notdir $$(tardir))
endef
$(eval $(call BuildBin,linux,amd64))
$(eval $(call BuildBin,linux,arm64))
$(eval $(call BuildBin,darwin,arm64))
$(eval $(call BuildBin,darwin,amd64))
$(eval $(call BuildBin,windows,amd64))
$(eval $(call BuildBin,freebsd,amd64))
$(eval $(call BuildBin,openbsd,amd64))
$(eval $(call BuildBin,netbsd,amd64))

build/bin:
	@mkdir -p $@
