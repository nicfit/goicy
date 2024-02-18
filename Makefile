build:
	go build goicy.go

clean:
	-rm goicy

test:
	go test ./...

all: clean build test