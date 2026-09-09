PREFIX ?= /usr/local

.PHONY: all build install clean

all: build

build:
	@echo "🔨 Building gbctl and gobackupd..."
	go build -o gbctl ./cmd/gbctl
	go build -o gobackupd ./cmd/gobackupd
	@echo "✅ Build complete!"

install: build
	@echo "📦 Installing to $(PREFIX)/bin..."
	install -d $(PREFIX)/bin
	install -m 755 gbctl $(PREFIX)/bin/
	install -m 755 gobackupd $(PREFIX)/bin/
	@echo "✅ Installation complete!"

clean:
	@echo "🧹 Cleaning up..."
	rm -f gbctl gobackupd
	@echo "✅ Clean complete!"
