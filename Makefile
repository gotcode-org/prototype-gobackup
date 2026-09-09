.PHONY: build run clean

build:
	@echo "🔨 Building gbctl and gobackupd..."
	go build -o gbctl ./cmd/gbctl
	go build -o gobackupd ./cmd/gobackupd
	@echo "✅ Build complete!"

clean:
	rm -f gbctl gobackupd
