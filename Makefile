GO ?= go
GOCACHE ?= /private/tmp/nexis-go-cache
BIN_DIR ?= bin

.PHONY: test build run-console run-node run-probe clean

test:
	GOCACHE=$(GOCACHE) $(GO) test ./...

build:
	mkdir -p $(BIN_DIR)
	GOCACHE=$(GOCACHE) $(GO) build -o $(BIN_DIR)/nexis-console ./apps/console
	GOCACHE=$(GOCACHE) $(GO) build -o $(BIN_DIR)/nexis-node ./agents/nexis-node
	GOCACHE=$(GOCACHE) $(GO) build -o $(BIN_DIR)/nexis-probe ./agents/nexis-probe

run-console:
	./start-console.sh

run-node:
	./start-node.sh

run-probe:
	./start-probe.sh

clean:
	rm -rf $(BIN_DIR)
