# ADR-0011 — Federação OIDC com os componentes do ArchGate

- **Status:** Aceito
- **Data:** 2026-07-19
- **Invariantes tocados:** I-1.1, I-4.5, I-7.6

## Contexto

O ArchGate compõe componentes heterogêneos que precisam confiar numa única fonte de identidade:

| Componente | Papel | Integração de identidade |
|---|---|---|
| **Warpgate** | Bastião SSH/HTTP/MySQL/Postgres | OIDC / SSO |
| **Apache Guacamole** | Acesso RDP/VNC/SSH via navegador | Extensão OpenID Connect |
| **NetBird** | Rede overlay WireGuard *zero trust* | OIDC (device flow e web) |
| **OpenBao** | Cofre de segredos | *auth method* OIDC/JWT |
| **Proxy Oracle JDBC** (Java, próprio) | Acesso a banco Consinco | Validação de JWT emitido pelo ArchGuard |

Sem contrato explícito, cada integração vira acordo ad-hoc: claims divergentes, semânticas de
grupo incompatíveis e impossibilidade de correlacionar auditoria entre planos.

## Decisão

**O ArchGuard é o único emissor de identidade do ArchGate. A integração é exclusivamente por
OIDC/OAuth2 padrão, sob um contrato de claims versionado (RFC-0006).**

### Princípios

1. **Nenhum componente acessa o banco do ArchGuard.** Integração só por protocolo.
2. **Contrato de claims versionado**, com `iss`, `sub` estável e opaco, `org` (tenant ativo),
   `acr`/`amr` (nível de garantia — ADR-0010), `groups`/`roles` normalizados, `sid` (sessão)
   e `act` quando houver delegação (ADR-0008).
3. **Correlação de auditoria**: um identificador de correlação de sessão privilegiada é
   propagado nos claims, permitindo unir o evento de autenticação (ArchGuard) ao evento de
   sessão (Warpgate/Guacamole) e ao acesso a segredo (OpenBao) em uma linha do tempo única.
   Sem isso, a trilha de auditoria do PAM fica fragmentada.
4. **Escopo mínimo e vida curta**: tokens de acesso curtos; refresh com rotação e detecção de
   reuso; toda emissão revogável (I-4.5).
5. **Back-channel logout (OIDC)**: encerrar a sessão no ArchGuard **encerra as sessões
   derivadas** nos componentes. Revogação de acesso que não propaga é revogação fictícia.
6. **Perfis de cliente por componente**: cada componente é uma aplicação registrada com
   *client* próprio, segredo custodiado no OpenBao (ADR-0012), redirecionamentos restritos e
   escopos mínimos — nunca um *client* compartilhado.
7. **Adaptação sem contaminação**: componente que não suporte plenamente um claim recebe
   adaptação em **camada de borda** do próprio componente, nunca por degradação do contrato
   central.

### Verificação de conformidade
Suíte de testes de federação que sobe cada componente contra o ArchGuard e valida:
login, propagação de claims, step-up recusado quando `acr` insuficiente, e logout
propagado. É gate de release (I-9.4).

## Alternativas consideradas

| Opção | Por que não |
|---|---|
| Integração por LDAP com cada componente | Perde `acr`/`amr`, contexto de tenant e logout propagado; LDAP fica como compatibilidade de borda para diretórios legados, não como espinha dorsal |
| Cada componente com sua própria base de usuários | Destrói a proposta de valor do ArchGate; torna desligamento de operador um processo manual e falível |
| Token compartilhado entre componentes | Viola escopo mínimo; comprometimento de um componente vira comprometimento de todos |

## Consequências

- Dependência de que cada componente honre corretamente o OIDC — divergências reais de
  implementação exigirão adaptadores e estão mapeadas como risco no RFC-0006.
- O ArchGuard vira ponto de disponibilidade crítica do ArchGate: exige HA e degradação
  documentada (sessões existentes sobrevivem à indisponibilidade do emissor; novas não).
