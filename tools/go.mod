// Módulo de ferramentas de CI — SEPARADO do go.mod principal (ADR-0002 §3a).
// As dependências das ferramentas (go-licenses, cyclonedx-gomod) NÃO entram na
// árvore de build do produto, no SBOM nem no scan de licenças. Versões fixadas.
module github.com/casdoor/casdoor/tools

go 1.25

require (
	github.com/CycloneDX/cyclonedx-gomod v1.9.0
	github.com/google/go-licenses v1.6.0
)
