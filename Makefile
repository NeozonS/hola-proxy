PROGNAME = hola-proxy
PKG = ./cmd/hola-proxy
OUTSUFFIX = bin/$(PROGNAME)
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo undefined)
BUILDOPTS = -a -tags netgo -trimpath -asmflags -trimpath
LDFLAGS = -ldflags '-s -w -extldflags "-static" -X main.appVersion=$(VERSION)'
LDFLAGS_NATIVE = -ldflags '-s -w -X main.appVersion=$(VERSION)'

NDK_CC_ARM = $(abspath ../../ndk-toolchain-arm/bin/arm-linux-androideabi-gcc)
NDK_CC_ARM64 = $(abspath ../../ndk-toolchain-arm64/bin/aarch64-linux-android21-clang)

GO := go

# All Go sources under cmd/ and internal/, plus module manifests.
src = $(shell find cmd internal -name '*.go' 2>/dev/null) go.mod go.sum

native: bin-native
all: bin-linux-amd64 bin-linux-386 bin-linux-arm bin-linux-arm64 \
	bin-linux-mips bin-linux-mipsle bin-linux-mips64 bin-linux-mips64le \
	bin-freebsd-amd64 bin-freebsd-386 bin-freebsd-arm \
	bin-netbsd-amd64 bin-netbsd-386 \
	bin-openbsd-amd64 bin-openbsd-386 \
	bin-darwin-amd64 bin-darwin-arm64 \
	bin-windows-amd64 bin-windows-386 bin-windows-arm64

allplus: all \
	bin-android-arm bin-android-arm64

bin-native: $(OUTSUFFIX)
bin-linux-amd64: $(OUTSUFFIX).linux-amd64
bin-linux-386: $(OUTSUFFIX).linux-386
bin-linux-arm: $(OUTSUFFIX).linux-arm
bin-linux-arm64: $(OUTSUFFIX).linux-arm64
bin-linux-mips: $(OUTSUFFIX).linux-mips
bin-linux-mipsle: $(OUTSUFFIX).linux-mipsle
bin-linux-mips64: $(OUTSUFFIX).linux-mips64
bin-linux-mips64le: $(OUTSUFFIX).linux-mips64le
bin-freebsd-amd64: $(OUTSUFFIX).freebsd-amd64
bin-freebsd-386: $(OUTSUFFIX).freebsd-386
bin-freebsd-arm: $(OUTSUFFIX).freebsd-arm
bin-netbsd-amd64: $(OUTSUFFIX).netbsd-amd64
bin-netbsd-386: $(OUTSUFFIX).netbsd-386
bin-openbsd-amd64: $(OUTSUFFIX).openbsd-amd64
bin-openbsd-386: $(OUTSUFFIX).openbsd-386
bin-darwin-amd64: $(OUTSUFFIX).darwin-amd64
bin-darwin-arm64: $(OUTSUFFIX).darwin-arm64
bin-windows-amd64: $(OUTSUFFIX).windows-amd64.exe
bin-windows-386: $(OUTSUFFIX).windows-386.exe
bin-windows-arm64: $(OUTSUFFIX).windows-arm64.exe
bin-android-arm: $(OUTSUFFIX).android-arm
bin-android-arm64: $(OUTSUFFIX).android-arm64

$(OUTSUFFIX): $(src)
	$(GO) build $(LDFLAGS_NATIVE) -o $@ $(PKG)

$(OUTSUFFIX).linux-amd64: $(src)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build $(BUILDOPTS) $(LDFLAGS) -o $@ $(PKG)

$(OUTSUFFIX).linux-386: $(src)
	CGO_ENABLED=0 GOOS=linux GOARCH=386 $(GO) build $(BUILDOPTS) $(LDFLAGS) -o $@ $(PKG)

$(OUTSUFFIX).linux-arm: $(src)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm $(GO) build $(BUILDOPTS) $(LDFLAGS) -o $@ $(PKG)

$(OUTSUFFIX).linux-arm64: $(src)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build $(BUILDOPTS) $(LDFLAGS) -o $@ $(PKG)

$(OUTSUFFIX).linux-mips: $(src)
	CGO_ENABLED=0 GOOS=linux GOARCH=mips GOMIPS=softfloat $(GO) build $(BUILDOPTS) $(LDFLAGS) -o $@ $(PKG)

$(OUTSUFFIX).linux-mips64: $(src)
	CGO_ENABLED=0 GOOS=linux GOARCH=mips64 GOMIPS=softfloat $(GO) build $(BUILDOPTS) $(LDFLAGS) -o $@ $(PKG)

$(OUTSUFFIX).linux-mipsle: $(src)
	CGO_ENABLED=0 GOOS=linux GOARCH=mipsle GOMIPS=softfloat $(GO) build $(BUILDOPTS) $(LDFLAGS) -o $@ $(PKG)

$(OUTSUFFIX).linux-mips64le: $(src)
	CGO_ENABLED=0 GOOS=linux GOARCH=mips64le GOMIPS=softfloat $(GO) build $(BUILDOPTS) $(LDFLAGS) -o $@ $(PKG)

$(OUTSUFFIX).freebsd-amd64: $(src)
	CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 $(GO) build $(BUILDOPTS) $(LDFLAGS) -o $@ $(PKG)

$(OUTSUFFIX).freebsd-386: $(src)
	CGO_ENABLED=0 GOOS=freebsd GOARCH=386 $(GO) build $(BUILDOPTS) $(LDFLAGS) -o $@ $(PKG)

$(OUTSUFFIX).freebsd-arm: $(src)
	CGO_ENABLED=0 GOOS=freebsd GOARCH=arm $(GO) build $(BUILDOPTS) $(LDFLAGS) -o $@ $(PKG)

$(OUTSUFFIX).netbsd-amd64: $(src)
	CGO_ENABLED=0 GOOS=netbsd GOARCH=amd64 $(GO) build $(BUILDOPTS) $(LDFLAGS) -o $@ $(PKG)

$(OUTSUFFIX).netbsd-386: $(src)
	CGO_ENABLED=0 GOOS=netbsd GOARCH=386 $(GO) build $(BUILDOPTS) $(LDFLAGS) -o $@ $(PKG)

$(OUTSUFFIX).openbsd-amd64: $(src)
	CGO_ENABLED=0 GOOS=openbsd GOARCH=amd64 $(GO) build $(BUILDOPTS) $(LDFLAGS) -o $@ $(PKG)

$(OUTSUFFIX).openbsd-386: $(src)
	CGO_ENABLED=0 GOOS=openbsd GOARCH=386 $(GO) build $(BUILDOPTS) $(LDFLAGS) -o $@ $(PKG)

$(OUTSUFFIX).darwin-amd64: $(src)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build $(BUILDOPTS) $(LDFLAGS) -o $@ $(PKG)

$(OUTSUFFIX).darwin-arm64: $(src)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO) build $(BUILDOPTS) $(LDFLAGS) -o $@ $(PKG)

$(OUTSUFFIX).windows-amd64.exe: $(src)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build $(BUILDOPTS) $(LDFLAGS) -o $@ $(PKG)

$(OUTSUFFIX).windows-386.exe: $(src)
	CGO_ENABLED=0 GOOS=windows GOARCH=386 $(GO) build $(BUILDOPTS) $(LDFLAGS) -o $@ $(PKG)

$(OUTSUFFIX).windows-arm64.exe: $(src)
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 $(GO) build $(BUILDOPTS) $(LDFLAGS) -o $@ $(PKG)

$(OUTSUFFIX).android-arm: $(src)
	CC=$(NDK_CC_ARM) CGO_ENABLED=1 GOOS=android GOARCH=arm GOARM=7 $(GO) build $(LDFLAGS_NATIVE) -o $@ $(PKG)

$(OUTSUFFIX).android-arm64: $(src)
	CC=$(NDK_CC_ARM64) CGO_ENABLED=1 GOOS=android GOARCH=arm64 $(GO) build $(LDFLAGS_NATIVE) -o $@ $(PKG)

clean:
	rm -f bin/*

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

test:
	$(GO) test ./...

tidy:
	$(GO) mod tidy

run:
	$(GO) run $(LDFLAGS_NATIVE) $(PKG)

install:
	$(GO) install $(LDFLAGS_NATIVE) $(PKG)

.PHONY: clean all allplus native fmt vet test tidy run install \
	bin-native \
	bin-linux-amd64 \
	bin-linux-386 \
	bin-linux-arm \
	bin-linux-arm64 \
	bin-linux-mips \
	bin-linux-mipsle \
	bin-linux-mips64 \
	bin-linux-mips64le \
	bin-freebsd-amd64 \
	bin-freebsd-386 \
	bin-freebsd-arm \
	bin-netbsd-amd64 \
	bin-netbsd-386 \
	bin-openbsd-amd64 \
	bin-openbsd-386 \
	bin-darwin-amd64 \
	bin-darwin-arm64 \
	bin-windows-amd64 \
	bin-windows-386 \
	bin-windows-arm64 \
	bin-android-arm \
	bin-android-arm64
