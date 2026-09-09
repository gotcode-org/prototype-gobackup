# Variables
BINARY_NAME=gobackup
INSTALL_PATH=/usr/local/bin/$(BINARY_NAME)

.PHONY: all build clean install run list-servers

# Default target
all: build

# Build the Go application
build:
	@echo "🔨 Building $(BINARY_NAME)..."
	go build -o $(BINARY_NAME) main.go
	@echo "✅ Build complete!"

# Run the default backup job
run: build
	@echo "🚀 Running $(BINARY_NAME)..."
	./$(BINARY_NAME)

# Test the list servers command
list-servers: build
	./$(BINARY_NAME) list servers

# Clean up the compiled binary
clean:
	@echo "🧹 Cleaning up..."
	go clean
	rm -f $(BINARY_NAME)
	@echo "✅ Clean complete!"

# Install the binary to /usr/local/bin (requires sudo)
install: build
	@echo "📦 Installing to $(INSTALL_PATH)..."
	sudo cp $(BINARY_NAME) $(INSTALL_PATH)
	sudo chmod +x $(INSTALL_PATH)
	@echo "✅ Installed successfully! You can now run '$(BINARY_NAME)' from anywhere."
