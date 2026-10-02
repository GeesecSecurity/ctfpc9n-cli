.PHONY: ctfpc9n-cli release install check-goctl sync-contracts generate check test verify clean

GOCTL_VERSION := 1.10.1
BUILD_DIR := .build
GENERATOR := $(BUILD_DIR)/ctfpc9n-api-gen
CONTRACT_DIR := contracts/runtime
CONTRACT := $(CONTRACT_DIR)/main.api
MANIFEST := contracts/agent-endpoints.json
GENERATED := internal/generated/agentapi

ctfpc9n-cli: test
	mkdir -p bin
	go build -o bin/ctfpc9n-cli ./cmd/ctfpc9n-cli

release: test
	mkdir -p bin
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o bin/ctfpc9n-cli-windows-amd64.exe ./cmd/ctfpc9n-cli
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/ctfpc9n-cli-linux-amd64 ./cmd/ctfpc9n-cli
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/ctfpc9n-cli-linux-arm64 ./cmd/ctfpc9n-cli
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o bin/ctfpc9n-cli-darwin-amd64 ./cmd/ctfpc9n-cli
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o bin/ctfpc9n-cli-darwin-arm64 ./cmd/ctfpc9n-cli

install: ctfpc9n-cli
	mkdir -p "$(HOME)/.local/bin"
	install -m 0755 bin/ctfpc9n-cli "$(HOME)/.local/bin/ctfpc9n-cli"

check-goctl:
	@version="$$(goctl --version 2>/dev/null | awk '{print $$3}')"; \
	if [ "$$version" != "$(GOCTL_VERSION)" ]; then \
		echo "ctfpc9n-cli requires goctl $(GOCTL_VERSION), found $${version:-missing}" >&2; \
		exit 1; \
	fi

sync-contracts:
	bash tools/sync-contracts.sh
	$(MAKE) check-goctl
	goctl api validate --api $(CONTRACT)

generate: generate-evaluation check-goctl $(GENERATOR)
	@test -f "$(CONTRACT)" || { echo "Missing competition contract; run make sync-contracts with RUNTIME_API_DIR first" >&2; exit 1; }
	@set -e; temporary="$$(mktemp -d)"; trap 'rm -rf "$$temporary"' EXIT; \
	goctl api plugin --api $(CONTRACT) --dir . --plugin="$(abspath $(GENERATOR))=--package agentapi --output $$temporary --manifest $(abspath $(MANIFEST))"; \
	mkdir -p $(GENERATED); cp "$$temporary"/*.go $(GENERATED)/

$(GENERATOR): tools/generate/go.mod tools/generate/go.sum tools/generate/main.go
	mkdir -p $(BUILD_DIR)
	go build -C tools/generate -o ../../$(GENERATOR) .

check:
	go test -mod=readonly ./...
	go vet -structtag=false ./...
	go vet -C tools/generate ./...

test: check

verify: generate check

clean:
	rm -rf $(BUILD_DIR) bin

.PHONY: generate-evaluation sync-evaluation-contracts
generate-evaluation: check-goctl $(GENERATOR)
	mkdir -p internal/generated/evaluationapi
	goctl api plugin --api contracts/evaluation/main.api --dir . --plugin="$(abspath $(GENERATOR))=--profile evaluation --package evaluationapi --output $(abspath internal/generated/evaluationapi) --manifest $(abspath contracts/evaluation-endpoints.json)"
sync-evaluation-contracts:
	bash tools/sync-evaluation-contracts.sh
