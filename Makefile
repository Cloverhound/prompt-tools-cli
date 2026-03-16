.PHONY: build check clean

build:
	go build -o prompt-tools .

check: build
	go vet ./...

clean:
	rm -f prompt-tools
