build:
	@go build

test:
	@AWS_SECRET_ACCESS_KEY=secret AWS_ACCESS_KEY_ID=secret go test -race -coverprofile=coverage.txt -covermode=atomic ./...

run: build
	@./go3up -bucket="s3.ungur.ro" -source=test/output -cachefile=test/.go3up.txt

cover:
	@AWS_SECRET_ACCESS_KEY=secret AWS_ACCESS_KEY_ID=secret go test -coverprofile=coverage.out
	@go tool cover -html=coverage.out

clean:
	@rm -f go3up coverage.out

.PHONY: test build
