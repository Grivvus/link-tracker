COVERAGE_FILE ?= coverage.out

# Get all directories in cmd/ as available modules
MODULES := $(notdir $(wildcard cmd/*))

# Help target - display usage information
.PHONY: help
help:
	@echo "Available commands:"
	@echo "  \033[36mmake build\033[0m - Build all modules ($(MODULES))"
	@$(foreach mod,$(MODULES),echo "  \033[36mmake build_$(mod)\033[0m - Build $(mod) module";)
	@echo "  \033[36mmake test\033[0m - Run all tests"

.PHONY: build
build:
	@echo "Building all modules: $(MODULES)"
	@mkdir -p bin
	@$(foreach mod,$(MODULES),echo "Building module: $(mod)"; go build -o ./bin/$(mod) ./cmd/$(mod);)

# Convenience targets for building individual modules
.PHONY: $(addprefix build_,$(MODULES))
$(addprefix build_,$(MODULES)):
	@modulename=$(subst build_,,$@); \
	echo "Building module: $$modulename"; \
	mkdir -p bin; \
	go build -o ./bin/$$modulename ./cmd/$$modulename

## test: run all unit tests
.PHONY: test-unit
test-unit:
	@go test -coverpkg='gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/...' --race -count=1 -coverprofile='$(COVERAGE_FILE)' ./internal/...
	@go tool cover -func='$(COVERAGE_FILE)' | grep ^total | tr -s '\t'

.PHONY: lint
lint:
	@echo "starting golangci-lint in docker"
	@docker run -t --rm -v $$(pwd):/app -w /app golangci/golangci-lint:v2.11.3 golangci-lint run

.PHONE: generate-api
generate-api:
	@echo "generating client, server and types for scrapper"
	@go tool oapi-codegen -generate "types" -package scrapper scrapper-api.yml > internal/infrastructure/openapi/scrapper/types.gen.go
	@go tool oapi-codegen -generate "std-http-server" -package scrapper scrapper-api.yml > internal/infrastructure/openapi/scrapper/server.gen.go
	@go tool oapi-codegen -generate "client" -package scrapper scrapper-api.yml > internal/infrastructure/openapi/scrapper/client.gen.go
	@echo "generating client, server and type for bot"
	@go tool oapi-codegen -generate "types" -package bot bot-api.yml > internal/infrastructure/openapi/bot/types.gen.go
	@go tool oapi-codegen -generate "std-http-server" -package bot bot-api.yml > internal/infrastructure/openapi/bot/server.gen.go
	@go tool oapi-codegen -generate "client" -package bot bot-api.yml > internal/infrastructure/openapi/bot/client.gen.go

.PHONY: generate-grpc
generate-grpc:
	@echo "generating scrapper grpc client, server and types"
	@protoc --go_out=internal/infrastructure/ --go_opt=paths=source_relative \
            --go-grpc_out=internal/infrastructure/ --go-grpc_opt=paths=source_relative \
            grpc/scrapper/scrapper.proto
	@echo "generating bot grpc client, server and types"
	@protoc --go_out=internal/infrastructure/ --go_opt=paths=source_relative \
            --go-grpc_out=internal/infrastructure/ --go-grpc_opt=paths=source_relative \
            grpc/bot/bot.proto

.PHONY: generate-sqlc
generate-sqlc:
	@echo "Generate sqlc"
	@go tool sqlc generate -f ./db/sqlc.yml
