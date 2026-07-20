# ADR-0002 — Regime de licenciamento, atribuição e higiene de dependências

- **Status:** Aceito
- **Data:** 2026-07-19
- **Invariantes tocados:** I-2.1 a I-2.4, I-8.1

## Contexto

O ArchGuard é um derivado de obra sob Apache License 2.0, distribuído como produto comercial
(SaaS e on-premises). A Apache 2.0 permite derivados proprietários, mas impõe obrigações
concretas frequentemente ignoradas em forks: preservação de avisos, arquivo `NOTICE`,
declaração de modificações e manutenção dos cabeçalhos de copyright.

Além disso, o setor de identidade/infraestrutura demonstrou entre 2023 e 2025 uma tendência
consistente de relicenciamento restritivo (HashiCorp → BUSL em 2023; Teleport → AGPL +
licença community restritiva; ZITADEL → AGPL-3.0 em 31/03/2025). Isso torna a higiene de
dependências uma questão de continuidade de negócio, não de conformidade formal.

## Decisão

### 1. Obrigações Apache 2.0 do fork
- `LICENSE` do upstream **preservado sem alteração**.
- `NOTICE` mantido e **acrescido** de bloco de atribuição do ArchGuard, contendo: obra
  original, autores originais, URL do repositório, **commit-base e tag do fork point**, e
  declaração de que arquivos foram modificados.
- Cabeçalhos de copyright existentes **não são removidos nem reescritos**. Arquivos
  modificados recebem linha adicional de modificação; arquivos **novos** recebem cabeçalho
  de copyright da IntegrAllTech.
- Marcas e nomes do upstream não são usados para promover o ArchGuard (Apache 2.0, §6).

### 2. Grant irrevogável
A concessão Apache 2.0 sobre o código já publicado é **irrevogável**. Um eventual
relicenciamento futuro do upstream **não afeta** o código já incorporado ao fork; afeta apenas
a capacidade de importar contribuições posteriores. Consequência operacional: **o fork point
é ativo de valor** e deve ser documentado com precisão forense.

### 3. Matriz de licenças permitidas

| Classe | Licenças | Uso permitido |
|---|---|---|
| **Permitida** | Apache-2.0, MIT, BSD-2/3, ISC, Unlicense, Zlib | Livre, inclusive linkagem estática |
| **Condicionada** | MPL-2.0, EPL-2.0, CDDL | **Somente** como serviço em processo separado (ex.: OpenBao), nunca linkado ao binário |
| **Proibida** | AGPL-*, GPL-*, LGPL-* (linkado), SSPL, BUSL, Elastic License, "Community Edition" com corte por porte de empresa | Bloqueio de build |
| **Revisão obrigatória** | CC-BY-*, EUPL, licenças duais, código sem licença declarada | Aprovação caso a caso registrada em PR |

### 4. Gate automatizado
- Geração de **SBOM (CycloneDX)** em todo build.
- **License gate** no CI: qualquer dependência (direta ou transitiva) fora da matriz
  **quebra o build**, sem exceção via flag.
- Auditoria trimestral de licenças das dependências existentes, cobrindo mudança de licença
  em versões novas.

### 5. Procedência de código
Código sem procedência licencial demonstrável não é incorporado — inclui trechos copiados de
fóruns, exemplos de documentação de terceiros e geração assistida por IA cuja origem não possa
ser justificada. Toda PR declara procedência.

### 6. Fiscal do upstream
Monitoramento automatizado (semanal) do arquivo `LICENSE` do upstream e dos anúncios de
release. Mudança de licença dispara **incidente de governança** com triagem em 48h.

## Consequências

- Custo fixo de CI (SBOM + gate) e disciplina de PR.
- Elimina classe inteira de risco jurídico e de continuidade.
- Restringe escolhas técnicas: bibliotecas úteis sob GPL/AGPL ficam indisponíveis para
  linkagem, forçando alternativas ou isolamento por processo.

## Nota

Este ADR é **panorama técnico-jurídico, não parecer legal**. A validação com advogado
especializado é pré-requisito do marco M1, com atenção especial a: obrigações MPL-2.0 de
dependências condicionadas, licenças de drivers de banco e compatibilidade de dependências
transitivas.
