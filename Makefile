.PHONY: fmt fmt_install lint lint_install test build clean

fmt:
	@gofumpt -l -w .
	@gofmt -s -w .
	@gci write --custom-order -s standard -s "prefix(github.com/sagernet/)" -s "default" .

fmt_install:
	go install -v mvdan.cc/gofumpt@latest
	go install -v github.com/daixiang0/gci@latest

lint:
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@v2.10.1 run ./...

lint_install:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v2.10.1

test:
	go test -v ./...

build:
	go build -o sing-geoip .

clean:
	rm -f sing-geoip geoip.db geoip-id.db *.sha256sum
	rm -rf rule-set release _push
