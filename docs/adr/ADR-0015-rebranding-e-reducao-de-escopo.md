# ADR-0015 — Rebranding, remoção de escopo e redução de superfície de ataque

- **Status:** Aceito
- **Data:** 2026-07-19
- **Invariantes tocados:** I-8.3, I-2.1, I-4.4

## Contexto

O upstream é um IAM generalista, com amplo catálogo de provedores de identidade, integrações
de pagamento, gestão de produtos/assinaturas, provedores de SMS/e-mail regionais, integrações
com ecossistemas específicos e, mais recentemente, funcionalidades voltadas a autenticação de
agentes de IA (MCP).

Para o ArchGuard, a maior parte disso é **superfície de ataque sem contrapartida**: código não
exercitado, dependências extras, endpoints expostos e custo permanente de rebase. Em produto de
segurança, funcionalidade não usada é passivo.

## Decisão

### 1. Rebranding completo
Nome, marca, domínios, identificadores de pacote, cabeçalhos de resposta e artefatos visuais
passam a ser do ArchGuard. Nenhuma referência à marca do upstream é usada para promover o
produto (Apache 2.0, §6) — enquanto a **atribuição de autoria** exigida permanece no `NOTICE`
(ADR-0002). Rebranding não é remoção de atribuição.

### 2. Remoção de escopo (out of scope, removido do build)
- Módulos de **pagamento, produto e assinatura**.
- Funcionalidades de **autenticação de agentes de IA/MCP** do upstream.
- Provedores de identidade e de mensageria **irrelevantes ao mercado-alvo** — mantém-se um
  conjunto curado (ver item 3).
- Recursos de "IAM genérico" sem uso em PAM que ampliem superfície administrativa.
- **Senha-mestra** (ADR-0008) — removida por invariante, não por escopo.

### 3. Catálogo curado de provedores (v1)
- **Corporativos**: Microsoft Entra ID / Active Directory (LDAP e OIDC), Google Workspace,
  Okta, IdPs SAML 2.0 genéricos, OIDC genérico.
- **Diretórios**: LDAP/AD via conector com sincronismo (RFC-0007).
- **Borda legada**: apenas o servidor RADIUS embutido. O **servidor LDAP embutido está fora
  do catálogo** — removido pelo ADR-0019 (aplicação do §5 deste ADR: sem caso de uso PAM em
  12 meses, superfície de ataque, dependência GPL). O conector *cliente* LDAP/AD acima não é
  afetado.
- Provedor novo entra apenas por demanda concreta de cliente e com dono de manutenção
  designado.

### 4. Endpoints e superfície
- Todo endpoint remanescente é inventariado, classificado (público / autenticado / L2 / L3) e
  coberto por teste de autorização. **Endpoint sem classificação não sobe.**
- Recursos habilitados por padrão são o mínimo funcional (I-4.4).

### 5. Critério de remoção
Remove-se o que: (a) não tem caso de uso em PAM no roadmap de 12 meses; (b) amplia superfície
de ataque; (c) traz dependência fora da matriz de licenças (ADR-0002). Toda remoção é
registrada em `DIVERGENCE.md`, para que cherry-picks futuros nessas áreas sejam descartados
automaticamente na triagem.

## Consequências

### Positivas
- Superfície de ataque e custo de manutenção significativamente menores.
- Auditoria de segurança (e futura certificação) sobre base menor e compreendida.
- Triagem de upstream mais barata: áreas removidas não geram fila.

### Negativas
- Perda de funcionalidades que poderiam atender nichos — **decisão consciente**: o ArchGuard é
  PAM, não IAM genérico.
- Remoções amplas aumentam a divergência e, portanto, o custo de rebase de major (ADR-0003).
- Necessário cuidado para que a remoção não quebre acoplamentos internos não óbvios do
  upstream: cada remoção é tarefa com verificação de build e testes.
