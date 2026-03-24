.PHONY: build build/ktha-node build/ktha-runner clean

build: build/ktha-node build/ktha-runner

build/ktha-node:
	go build -o build/ktha-node ./cmd/ktha-node

build/ktha-runner:
	go build -o build/ktha-runner ./cmd/ktha-runner

clean:
	rm -rf build/
