# Makefile — ArchGuard
# Gate de verificação (CLAUDE.md §5): lint → test → invariants → deps-check → sbom → build
# Bloco 0/1: invariants, deps-check e sbom são stubs que FALHAM até T-018/T-019 (gate parcial
# esperado e aceito — ver openspec/changes/001-bootstrap-fork/tasks.md).

.PHONY: gate lint test invariants deps-check sbom build upstream-triage

gate: lint test invariants deps-check sbom build

# Triagem semanal de upstream (ADR-0003): atualiza o espelho somente-leitura
# vendor/upstream e emite a fila de triagem classificada. NUNCA faz merge em main.
upstream-triage:
	git fetch upstream master --tags
	git branch -f vendor/upstream upstream/master
	go run -C tools ./upstreamwatch "$(CURDIR)"

lint:
	@test -f go.mod || { echo "lint: árvore Go ausente (go.mod não encontrado). O fork point é congelado em T-002."; exit 1; }
	gofmt -l . | (! grep .) || { echo "lint: arquivos fora de formatação (gofmt)"; exit 1; }
	go run -C tools ./lintbaseline "$(CURDIR)"

test:
	@test -f go.mod || { echo "test: árvore Go ausente (go.mod não encontrado). O fork point é congelado em T-002."; exit 1; }
	go test ./...

invariants:
	go test ./test/invariants/ -count=1 -timeout 30m

deps-check:
	go test ./test/invariants/ -count=1 -run 'TestINV3|TestSelfINV3'

sbom:
	go run -C tools github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod mod -licenses -json -output "$(CURDIR)/sbom.json" "$(CURDIR)"
	go run -C tools ./licensegate "$(CURDIR)" "$(CURDIR)/sbom.json"

build:
	@test -f go.mod || { echo "build: árvore Go ausente (go.mod não encontrado). O fork point é congelado em T-002."; exit 1; }
	go build ./...
