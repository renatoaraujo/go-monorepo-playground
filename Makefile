GO_BUILD_ARGS = -ldflags "-X main.version=${VERSION}"

DEFAULT_TARGET = all

#@ Helpers
# from https://www.thapaliya.com/en/writings/well-documented-makefiles/
help:  ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Tools
tools: ## Installs required binaries locally
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

build:
	@echo "== build"
	CGO_ENABLED=0 go build $(GO_BUILD_ARGS) -o ./bin/ .

##@ Load Testing
loadtest: ## Run comprehensive load tests against services
	@echo "== running comprehensive load tests"
	@mkdir -p ./loadtest-results
	@echo "Testing producer service..."
	@vegeta attack -targets=./tools/vegeta-targets.txt -rate=50 -duration=2m -timeout=30s | vegeta report -type=text | tee ./loadtest-results/comprehensive-report.txt
	@echo "✓ Load test completed. Results saved to ./loadtest-results/comprehensive-report.txt"

loadtest-continuous: ## Run continuous load tests for sustained telemetry (light->medium->heavy->cool down)
	@echo "== running continuous load tests for telemetry generation"
	@mkdir -p ./loadtest-results
	@echo "Phase 1: Light load (10 req/sec for 3m)..."
	@vegeta attack -targets=./tools/vegeta-targets.txt -rate=10 -duration=3m -timeout=30s | vegeta report -type=text | tee ./loadtest-results/continuous-light.txt
	@echo "Phase 2: Medium load (30 req/sec for 3m)..."
	@vegeta attack -targets=./tools/vegeta-targets.txt -rate=30 -duration=3m -timeout=30s | vegeta report -type=text | tee ./loadtest-results/continuous-medium.txt
	@echo "Phase 3: Heavy load (100 req/sec for 3m)..."
	@vegeta attack -targets=./tools/vegeta-targets.txt -rate=100 -duration=3m -timeout=30s | vegeta report -type=text | tee ./loadtest-results/continuous-heavy.txt
	@echo "Phase 4: Cool down (10 req/sec for 3m)..."
	@vegeta attack -targets=./tools/vegeta-targets.txt -rate=10 -duration=3m -timeout=30s | vegeta report -type=text | tee ./loadtest-results/continuous-cooldown.txt
	@echo "✓ Continuous load test completed. Check Grafana dashboards at http://localhost:3000"

loadtest-light: ## Run light load test (10 req/sec for 1 minute)
	@echo "== running light load test"
	@mkdir -p ./loadtest-results
	@vegeta attack -targets=./tools/vegeta-targets.txt -rate=10 -duration=1m -timeout=30s | vegeta report -type=text | tee ./loadtest-results/light-report.txt

loadtest-heavy: ## Run heavy load test (200 req/sec for 30 seconds)
	@echo "== running heavy load test"
	@mkdir -p ./loadtest-results
	@vegeta attack -targets=./tools/vegeta-targets.txt -rate=200 -duration=30s -timeout=30s | vegeta report -type=text | tee ./loadtest-results/heavy-report.txt

loadtest-producer: ## Run load test against producer service only
	@echo "== running producer-only load test"
	@mkdir -p ./loadtest-results
	@echo "GET http://localhost:8080" | vegeta attack -rate=50 -duration=1m -timeout=30s | vegeta report -type=text | tee ./loadtest-results/producer-report.txt

loadtest-consumer: ## Run load test against consumer service only
	@echo "== running consumer-only load test"
	@mkdir -p ./loadtest-results
	@echo "GET http://localhost:8901" | vegeta attack -rate=50 -duration=1m -timeout=30s | vegeta report -type=text | tee ./loadtest-results/consumer-report.txt

loadtest-plot: ## Generate HTML plots from previous load test results
	@echo "== generating HTML plots from results"
	@mkdir -p ./loadtest-results
	@if [ -f ./loadtest-results/latest-results.bin ]; then \
		vegeta plot < ./loadtest-results/latest-results.bin > ./loadtest-results/plot.html && \
		echo "✓ HTML plot generated: ./loadtest-results/plot.html"; \
	else \
		echo "No binary results found. Run load tests with binary output first."; \
	fi

loadtest-with-plot: ## Run load test and generate HTML plot
	@echo "== running load test with plot generation"
	@mkdir -p ./loadtest-results
	@vegeta attack -targets=./tools/vegeta-targets.txt -rate=50 -duration=1m -timeout=30s > ./loadtest-results/latest-results.bin
	@vegeta report < ./loadtest-results/latest-results.bin | tee ./loadtest-results/latest-report.txt
	@vegeta plot < ./loadtest-results/latest-results.bin > ./loadtest-results/latest-plot.html
	@echo "✓ Load test completed with plot: ./loadtest-results/latest-plot.html"

loadtest-vegeta: ## Run vegeta with custom targets file
	@echo "== running vegeta with targets file"
	@vegeta attack -targets=./tools/vegeta-targets.txt -rate=50 -duration=1m | vegeta report

##@ Cleanup
clean: ## Deletes binaries from the bin folder
	@echo "== clean"
	@rm -rfv ./bin

clean-loadtest: ## Remove load test results directory
	@echo "== cleaning load test results"
	@rm -rfv ./loadtest-results

clean-all: clean clean-loadtest ## Clean binaries and load test results
	@echo "== clean all completed"

##@ Tests
test: tools ## Run unit tests
	@echo "== unit test"
	go test -cover ./...

##@ Run static checks
check: tools
	go fmt ./...
	go vet ./...
	golangci-lint run --config=.golangci.yaml ./...
