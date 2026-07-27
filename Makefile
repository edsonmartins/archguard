# Makefile — ArchGuard
# Gate de verificação (CLAUDE.md §5): lint → test → invariants → deps-check → sbom → build
# Bloco 0/1: invariants, deps-check e sbom são stubs que FALHAM até T-018/T-019 (gate parcial
# esperado e aceito — ver openspec/changes/001-bootstrap-fork/tasks.md).

.PHONY: gate release-gate lint test invariants deps-check sbom build conformance upstream-triage

gate: lint test invariants deps-check sbom build

# Gate de RELEASE (pacote 006, RFC-0006 §8): o gate de verificação + a suíte de
# conformidade OIDC por componente. Falha em qualquer item de conformidade
# BLOQUEIA o release (I-9.4). É este alvo que o CI roda para liberar uma release.
release-gate: gate conformance

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

# Suíte de conformidade OIDC por componente (pacote 006 T-016/T-017): valida o
# contrato de federação (claims, acr, rotação de chave, logout, correlação pcid)
# para cada componente registrado. Gate de release — falha bloqueia a liberação.
conformance:
	go test ./internal/adapters/oidc/ -count=1 -run 'Conformance|SignAndVerify|Rotation|UnknownKID|SignLogout|Acceptance'

deps-check:
	go test ./test/invariants/ -count=1 -run 'TestINV3|TestSelfINV3'

sbom:
	go run -C tools github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod mod -licenses -json -output "$(CURDIR)/sbom.json" "$(CURDIR)"
	go run -C tools ./licensegate "$(CURDIR)" "$(CURDIR)/sbom.json"

build:
	@test -f go.mod || { echo "build: árvore Go ausente (go.mod não encontrado). O fork point é congelado em T-002."; exit 1; }
	go build ./...

# E2E smoke do console PAM (pacote 008, T-020 Fase A — fluxos L1, sem step-up). Requer a
# stack sob teste no ar em ARCHGUARD_E2E_URL (default http://localhost:8000). Suba a stack
# com `make e2e-up` (compose em deploy/e2e/) OU aponte para um backend+console já rodando.
e2e:
	cd web && yarn run e2e:archguard

# Sobe/derruba a stack de E2E (Postgres + imagem do fork em perfil dev, servindo :8000).
e2e-up:
	docker compose -f deploy/e2e/docker-compose.e2e.yml up -d --wait
e2e-down:
	docker compose -f deploy/e2e/docker-compose.e2e.yml down -v
