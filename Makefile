# 对标 mysqld_exporter 的构建脚本。
# 产出 CGO_ENABLED=0 的静态 Linux 二进制，可直接在 RHEL/CentOS 8 (amd64) 运行，
# 不依赖目标机 glibc 版本与外部 C 库。

GOOS      ?= linux
GOARCH    ?= amd64
PREFIX    ?= oem-emcc-exporter
VERSION   ?= $(shell git describe --tags --always 2>/dev/null || echo dev)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X main.version=$(VERSION) \
	-X main.buildDate=$(BUILD_DATE)

PKGS := ./...

.PHONY: all build test vet lint tarball clean

all: build

# 编译静态二进制到当前目录
build:
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) \
	go build -ldflags "$(LDFLAGS)" -o $(PREFIX) .

test:
	go test $(PKGS)

vet:
	go vet $(PKGS)

# 打包二进制 + 配置示例 + README，供 Linux 8 部署
tarball: build
	@mkdir -p $(PREFIX)-$(VERSION).linux-$(GOARCH)
	cp $(PREFIX) oem-exporter.yml README.md \
		$(PREFIX)-$(VERSION).linux-$(GOARCH)/
	tar -czf $(PREFIX)-$(VERSION).linux-$(GOARCH).tar.gz \
		$(PREFIX)-$(VERSION).linux-$(GOARCH)/
	@echo "=> $(PREFIX)-$(VERSION).linux-$(GOARCH).tar.gz"

clean:
	rm -rf $(PREFIX) $(PREFIX)-*
