.PHONY: check fmt tidy vet build test race docker-check docker-check-go25 docker-check-go26

LIVE_TEST_ENV = LIBDNS_SPACESHIP_RUN_LIVE=0 LIBDNS_SPACESHIP_CLEANUP=0

check: fmt tidy vet build test

fmt:
	test -z "$$(gofmt -l .)"

tidy:
	$(LIVE_TEST_ENV) go mod tidy -diff

vet:
	$(LIVE_TEST_ENV) go vet ./...

build:
	$(LIVE_TEST_ENV) go build ./...

test:
	$(LIVE_TEST_ENV) go test -count=1 ./...

race:
	$(LIVE_TEST_ENV) go test -race -count=1 ./...

docker-check: docker-check-go25 docker-check-go26

docker-check-go25:
	docker run --rm -e LIBDNS_SPACESHIP_RUN_LIVE=0 -e LIBDNS_SPACESHIP_CLEANUP=0 -v "$(CURDIR):/src" -w /src golang:1.25 bash -c 'test -z "$$(gofmt -l .)" && go mod tidy -diff && go vet ./... && go test -race -count=1 ./... && go build ./...'

docker-check-go26:
	docker run --rm -e LIBDNS_SPACESHIP_RUN_LIVE=0 -e LIBDNS_SPACESHIP_CLEANUP=0 -v "$(CURDIR):/src" -w /src golang:1.26 bash -c 'test -z "$$(gofmt -l .)" && go mod tidy -diff && go vet ./... && go test -race -count=1 ./... && go build ./...'
