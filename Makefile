# Makefile — ArchGuard
# Gate de verificação (CLAUDE.md §5): lint → test → invariants → deps-check → sbom → build
# Bloco 0/1: invariants, deps-check e sbom são stubs que FALHAM até T-018/T-019 (gate parcial
# esperado e aceito — ver openspec/changes/001-bootstrap-fork/tasks.md).

.PHONY: gate lint test invariants deps-check sbom build

gate: lint test invariants deps-check sbom build

lint:
	@test -f go.mod || { echo "lint: árvore Go ausente (go.mod não encontrado). O fork point é congelado em T-002."; exit 1; }
	gofmt -l . | (! grep .) || { echo "lint: arquivos fora de formatação (gofmt)"; exit 1; }
	go vet ./...

test:
	@test -f go.mod || { echo "test: árvore Go ausente (go.mod não encontrado). O fork point é congelado em T-002."; exit 1; }
	go test ./...

invariants:
	go test ./test/invariants/ -count=1 -timeout 30m

deps-check:
	go test ./test/invariants/ -count=1 -run 'TestINV3|TestSelfINV3'

sbom:
	@echo "sbom: NÃO IMPLEMENTADO — SBOM CycloneDX + license gate (INV-4) é entregue em T-019."
	@exit 1

build:
	@test -f go.mod || { echo "build: árvore Go ausente (go.mod não encontrado). O fork point é congelado em T-002."; exit 1; }
	go build ./...
