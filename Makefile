export SHELLCHECK_ARGS = --enable=all -x -S style

.DEFAULT_GOAL := all

all: fmt lint actionlint vulncheck osv-scanner gitleaks deadcode shellcheck fixme test

test:
	@CGO_ENABLED=1 AWS_PROFILE= WORKSPACE=test go test -race -vet all -coverprofile=unit.cov -covermode=atomic $(OPTS) ./...
	@go tool cover -func=unit.cov|tail -n1
	@go tool -modfile=tools/go.mod stampli -quiet -coverage=$$(go tool cover -func=unit.cov|tail -n1|tr -s "\t"|cut -f3|tr -d "%")

lint:
	@go tool -modfile=tools/go.mod golangci-lint config verify
	@go tool -modfile=tools/go.mod golangci-lint run

update: update-go
	@go get -u -t ./...
	@go mod tidy
	@go generate ./...
	@(cd tools && go get tool && go mod tidy)

# Find all go.mod files and update the version to latest stable.
update-go:
	@LATEST=$$(go tool -modfile tools/go.mod eol latest go -t '{{.latest.name}}'); \
	  for i in $$(find . -name 'go.mod'); do \
	    go mod edit -go=$$LATEST -modfile=$$i; \
	  done

update-tf:
	@go tool -modfile tools/go.mod eol latest terraform -t '{{.latest.name}}' > deploy/.terraform-version
	@(cd deploy && terraform init -upgrade)
	@make -s tfdocs

lint-%:
	@go tool -modfile=tools/go.mod golangci-lint --enable-only="$(patsubst lint-%,%,$@)" run

fixme:
	@git grep FIXME: ":(exclude)Makefile" || git grep TODO: ":(exclude)Makefile" ":(exclude).golangci.yml" || true

deadcode:
	@go tool -modfile=tools/go.mod deadcode -test ./...

dockerlint:
	@set -e; for file in $$(find . -path '*/testdata/*' -prune -o \( -iname "dockerfile*" -o -iname "containerfile*" \) -print); do \
		echo "Linting $$file"; \
		podman run --rm -i --network=none docker.io/hadolint/hadolint hadolint -t style - < "$$file"; \
	done

shellcheck:
	@find . -path '*/testdata/*' -prune -o -type f -name '*.sh' -exec shellcheck $(SHELLCHECK_ARGS) {} +

tflint:
	@go tool -modfile=tools/go.mod tflint --init --chdir deploy
	@go tool -modfile=tools/go.mod tflint -f compact --chdir deploy --recursive

actionlint:
	@go tool -modfile=tools/go.mod actionlint -shellcheck="shellcheck $(SHELLCHECK_ARGS)"

tfdocs:
	@go tool -modfile=tools/go.mod terraform-docs md deploy > deploy/README.md

vulncheck:
	@go tool -modfile=tools/go.mod govulncheck ./...

osv-scanner:
	@go tool -modfile=tools/go.mod osv-scanner scan --recursive --experimental-exclude=tools .

gitleaks:
	@go tool -modfile=tools/go.mod gitleaks git . --no-banner $(OPTS)

fmt:
	@for d in $$(find . -name 'go.mod' -not -path './tools/go.mod' -exec dirname {} \;); do \
		(cd "$$d" && go fmt ./...); \
	done

coverage_map: test
	@echo '<?xml version="1.0" encoding="UTF-8"?>' > unit.svg
	@go tool -modfile=tools/go.mod go-cover-treemap -coverprofile unit.cov >> unit.svg

clean:
	@rm -rf *.cov
