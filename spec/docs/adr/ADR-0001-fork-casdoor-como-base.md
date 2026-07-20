# ADR-0001 — Fork direto do Casdoor como base do ArchGuard

- **Status:** Aceito
- **Data:** 2026-07-19
- **Decisores:** Edson Martins (Arquiteto de Software e Soluções), Neimar Chagas (sócio técnico/financeiro)
- **Invariantes tocados:** I-1.1, I-1.2, I-2.1, I-7.2
- **Substitui:** PoC "camada sobre Kanidm" (descontinuada)

## Contexto

O ArchGate (PAM) compõe Warpgate, OpenBao, Apache Guacamole, NetBird e um proxy Oracle JDBC
próprio em Java. Falta o plano de controle de identidade: o **ArchGuard**.

A primeira tentativa foi um wrapper (TanStack Start + React) sobre o **Kanidm** (Rust,
MPL-2.0). O resultado foi rejeitado por razões estruturais, não estéticas:

1. **Ausência de API de administração estável.** A documentação oficial de suporte do Kanidm
   declara estáveis apenas algumas APIs, tratando as demais interações como instáveis e
   exigindo paridade exata de versão entre CLI/conectores e servidor. Um plano de controle
   não pode ser construído sobre superfície declarada instável.
2. **SCIM apenas de entrada.** A doc oficial afirma que SCIM é suportado somente para
   sincronização *a partir de* outro IdP. Provisioning genérico de usuários/grupos por API —
   requisito duro de PAM B2B — não existe.
3. **Administração CLI-first.** Provisioning declarativo depende de ferramenta de terceiro
   (`kanidm-provision`) que historicamente requereu **patch no código-fonte** do servidor para
   provisionar segredos de clientes OAuth2. Isso é o sintoma definitivo de um produto que não
   foi projetado para ser embutido.
4. **Modelo de dados opinativo e auto-curativo**, sem multi-tenancy de primeira classe.
5. **Licença MPL-2.0** (copyleft por arquivo): evoluir o núcleo obriga a manter aberto o
   derivado, atritando com produto proprietário.

Conclusão: o Kanidm é um **IdP terminal**, projetado para consumo final, não como base de
plataforma. O padrão que funcionou na IntegrAllTech foi o do **DeskLenz**: fork direto de
projeto Apache 2.0 com assunção integral de governança.

## Decisão

**Adotar o Casdoor (Go + React, Apache License 2.0, sem CLA) como base de fork do ArchGuard.**

O fork assume governança total: CONSTITUTION, ADRs, RFCs e pacotes OpenSpec próprios. O
upstream passa a ser fonte de correções de segurança e melhorias pontuais, não direção de
produto.

## Alternativas consideradas

| Opção | Licença | Por que não |
|---|---|---|
| **Manter wrapper sobre Kanidm** | MPL-2.0 | API de administração instável; SCIM só de entrada; provisioning exige patch; sem multi-tenancy; copyleft por arquivo |
| **Keycloak** (fork) | Apache 2.0 | Vantagem real: Java, alinhado ao stack; realms = multi-tenancy maduro. Rejeitado por **massa crítica**: base Quarkus enorme, custo de manutenção de fork alto, console admin difícil de substituir sem quebrar fluxos internos. Mantido como **plano B formal** (ver "Reversibilidade") |
| **Ory (Kratos/Hydra/Keto)** | Apache 2.0 + Enterprise fechada | SCIM, SAML e multi-tenancy/organizations ficam sob licença enterprise fechada. Reimplementar essas features anula a vantagem do fork. Kratos é headless — nenhuma UI aproveitável |
| **ZITADEL** | **AGPL-3.0** (v3, 31/03/2025) | Eliminado por licença: derivado oferecido como serviço obrigaria abertura do código |
| **Teleport** | AGPLv3 + Community License restritiva | Eliminado por licença, apesar de ser o produto PAM mais próximo funcionalmente |
| **HashiCorp Boundary** | BUSL 1.1 | Não é open source; proíbe uso comercial competitivo e embedding |
| **Authentik** | MIT + `enterprise/` proprietário | Python+Go: fora do stack; open-core nas features B2B |
| **Rauthy** | Apache-2.0 | Excelente engenharia (Rust), mas OIDC-only: sem SAML, LDAP server, RADIUS ou multi-tenancy. Insuficiente como plano de controle |
| **Gravitee AM / Apereo CAS + OpenFGA** | Apache 2.0 | Java, viável, mas "menos produto pronto": exigiria montar gestão de usuários, console e protocolos de borda. Mantido como plano C |
| **Construir do zero** | — | Custo proibitivo e risco criptográfico: reimplementar OIDC/SAML/WebAuthn corretamente é anos de trabalho |

## Justificativa da escolha

O Casdoor é o único candidato que reúne simultaneamente:

- **Licença Apache 2.0 verificada e sem CLA** (inbound=outbound) — fork proprietário limpo;
- **Cobertura de protocolos superior a qualquer par permissivo**: OIDC/OAuth2, SAML 2.0
  (IdP e SP), servidor LDAP embutido, servidor RADIUS embutido, CAS 1/2/3, SCIM 2.0,
  WebAuthn/passkeys, TOTP — a borda de integração do ArchGate está toda coberta;
- **PostgreSQL como cidadão de primeira classe**;
- **API REST pública completa**: o console atual consome *apenas* a API pública, sem API
  privada de UI — o que torna a substituição do frontend um recorte limpo (ADR-0004);
- **Casbin embutido** como PDP coarse-grained, com integração natural a OpenFGA para o
  granular (ADR-0005);
- **Tamanho de base gerenciável** por uma squad, ao contrário do Keycloak.

## Consequências

### Positivas
- Time-to-market drasticamente menor que Keycloak-fork ou greenfield.
- Protocolos de borda (LDAP/RADIUS) prontos — críticos para clientes brasileiros com AD legado.
- Recorte de reescrita bem delimitado: console, auditoria, multi-tenancy, privilégio.

### Negativas / dívidas assumidas
- **Framework Beego**: momentum modesto no ecossistema Go, menor pool de contratação
  (mitigação em ADR-0016).
- **Auditoria não imutável** no upstream (endereçado em ADR-0007).
- **Superfície de risco de PAM no upstream**: master password e impersonation com
  registro insuficiente (endereçado em ADR-0008).
- **Multi-tenancy limitado**: um usuário pertence a exatamente uma organização
  (endereçado em ADR-0006).
- **Divergência do upstream**: a direção recente do Casdoor tende a autenticação para
  agentes de IA/MCP, que não é o eixo do ArchGuard — aumenta o custo de rebase e reforça a
  decisão de governança própria (ADR-0003).
- Cadência altíssima de micro-releases do upstream torna o rebase caro se não for seletivo.

## Reversibilidade

Custo de reversão **alto após o marco M3** (multi-tenancy + auditoria). Até M2, migrar para
Keycloak custaria a reescrita do console e dos conectores de borda, preservando-se os
contratos OIDC (RFC-0006), que são deliberadamente agnósticos de fornecedor. Essa é a razão
de o RFC-0006 existir: **manter o custo de troca de IdP contido nos contratos**.

## Ações imediatas

1. Congelar o **fork point** (commit-base + tag do upstream) e registrá-lo no `NOTICE`.
2. Executar due diligence jurídica de licença com advogado (validação, não parecer deste ADR).
3. Verificar na aba *Insights* do repositório upstream a release corrente e a base de
   mantenedores antes do congelamento — há divergência de fontes sobre a última tag.
