[doc("see this help text")]
help:
  @just -l


[doc("Lint the code")]
lint:
  golangci-lint run
  go fmt ./...
  go vet ./...


[doc("Test the code")]
test:
  go test ./...
