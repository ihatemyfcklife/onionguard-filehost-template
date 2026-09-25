.PHONY: all build run test clean tor-help

BINARY_NAME=bin/filehost

all: build

build:
	@mkdir -p bin
	go build -ldflags="-s -w" -o $(BINARY_NAME) ./cmd/server

run: build
	./$(BINARY_NAME)

test:
	go test -v -race ./...

clean:
	rm -rf bin data/uploads/* data/meta.json*

tor-help:
	@echo "================ Tor Onion Service Setup ================"
	@echo "1. Install tor daemon: sudo apt install tor"
	@echo "2. Edit /etc/tor/torrc:"
	@echo "     HiddenServiceDir /var/lib/tor/filehost_service/"
	@echo "     HiddenServicePort 80 127.0.0.1:8080"
	@echo "3. Restart tor daemon: sudo systemctl restart tor"
	@echo "4. Retrieve your .onion address:"
	@echo "     sudo cat /var/lib/tor/filehost_service/hostname"
	@echo "========================================================="
