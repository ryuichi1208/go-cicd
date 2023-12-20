TARGET=go-cicd

build:
	go build -ldflags \
		" \
		-X main.version=$(shell git describe --tag --abbrev=0) \
		-X main.revision=$(shell git rev-list -1 HEAD) \
		-X main.build=$(shell git describe --tags) \
		" \
		-o $(TARGET) .

clean:
	rm -f $(TARGET)
