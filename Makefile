TARGET=go-cicd

.PHONY:
build:
	go build -ldflags \
		" \
		-X main.version=$(shell git describe --tag --abbrev=0) \
		-X main.revision=$(shell git rev-list -1 HEAD) \
		-X main.build=$(shell git describe --tags) \
		" \
		-o $(TARGET) .

.PHONY:
clean:
	rm -f $(TARGET)

.PHONY: release
release: deps-release
	goreleaser --clean

.PHONY: deps-release
deps-release: goreleaser

.PHONY: goreleaser
goreleaser:
ifeq ($(shell command -v goreleaser 2> /dev/null),)
	go install github.com/goreleaser/goreleaser@latest
endif
