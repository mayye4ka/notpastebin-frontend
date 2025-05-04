include .env.local
export
build:
	go build ./cmd/...
run: build
	./notpastebin-frontend