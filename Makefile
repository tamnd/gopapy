.PHONY: test compat-build compat-run compat

# Run the unit tests and grammar fixture oracle (current Python).
test:
	go test ./...
	./tests/run.sh

# Build the compat Docker image.
compat-build:
	docker build -f Dockerfile.compat -t gopapy-compat .

# Run compat tests inside the already-built image.
compat-run:
	docker run --rm gopapy-compat

# Build and run in one step.
compat: compat-build compat-run
