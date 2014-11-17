build:
	@go build

test:
	@go test ./...

run: build
	@./go3up -bucket="s3.ungur.ro" -source=test/output -cachefile=test/.go3up.txt

cover:
	@go test -coverprofile=coverage.out
	@go tool cover -html=coverage.out

clean:
	@rm -f go3up coverage.out

.PHONY: test build
