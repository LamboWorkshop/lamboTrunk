LAMBOTRUNK := lamboTrunk
PKG_LIST := main.go
BUILD_DIR := "build"
COMMIT = $(shell git rev-list -1 HEAD)

.PHONY: all dep build clean test coverage coverhtml lint

all: build

run: build
	./build/$(LAMBOTRUNK)

lint: ## Lint the files
	@$(HOME)/go/bin/staticcheck ./...

dep: ## Get the dependencies
	go mod tidy

build: dep ## Build the binary file
	mkdir -p $(BUILD_DIR)
	go build -o build/$(LAMBOTRUNK) $(PKG_LIST)

prod: dep
	env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o build/$(LAMBOTRUNK) $(PKG_LIST)

clean: ## Remove previous build
	@rm -fr $(BUILD_DIR)

help: ## Display this help screen
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
