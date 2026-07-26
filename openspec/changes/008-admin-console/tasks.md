# Tasks — 008 · ArchGuard Console (evolução do console herdado)

> Base normativa: **ADR-0020** (ratificado 2026-07-26, supersede o ADR-0004). Este re-escopo troca
> "reescrever o console" por "**evoluir o console herdado**" (CRA + antd, já rebrandizado/
> tematizado), construindo nele as telas de PAM que faltam, contra o `/api/v1` (pacote 011).
> (Greenfield anterior no histórico git.)

## Fundação e travas (mantêm os invariantes do ADR-0004)
- [ ] **T-001** Camada de API tipada do console para o `/api/v1` (plano de controle, pacote 011):
      módulo dedicado; nenhuma chamada crua a `fetch` de PAM fora dele.
- [ ] **T-002** Teste de contrato no CI: as chamadas do console ao `/api/v1` conferem com o
      OpenAPI publicado (detecta *drift* contrato↔console). Falha o CI se defasar.
- [ ] **T-003** Trava de revisão/lint: nenhum endpoint "só para a UI" (I-7.6); nenhuma decisão de
      autorização no frontend.

## Contexto de operação (cross-cutting)
- [ ] **T-004** Seletor de tenant permanente no cabeçalho com distinção visual inequívoca do
      tenant ativo; troca reemite token e dispara step-up se a política do destino for mais
      restritiva.
- [ ] **T-005** Interceptor global de step-up: captura garantia insuficiente, apresenta desafio
      WebAuthn e **retoma a operação** preservando o estado do formulário; cancelar mantém o form.

## Telas críticas de PAM (o que o herdado não tem)
- [ ] **T-006** Concessões vigentes (privileged grants) com contagem regressiva e revogação.
- [ ] **T-007** Solicitação de break-glass com justificativa e incidente.
- [ ] **T-008** Fila de aprovação de break-glass (separação de deveres; sem autoaprovação).
- [ ] **T-009** Timeline de auditoria com filtros + **indicador de integridade da cadeia sempre
      visível**, com divergência em destaque máximo; acionamento da verificação (L3).
- [ ] **T-010** Visão de correlação por `pcid` (ArchGuard + componentes); ator real vs sujeito em
      delegação.
- [ ] **T-011** Exportação assinada da trilha (L3).
- [ ] **T-012** Campanhas de revisão de acesso: acesso efetivo do PDP com origem
      (direto/herdado/concessão); decisões em lote, cada uma auditada.
- [ ] **T-013** Saúde dos subsistemas (PDP, cofre, auditoria) — agregado honesto (sem verde no
      topo com divergência no detalhe).
- [ ] **T-014** Chaves e rotação (L3).

## UX, segurança e conformidade
- [ ] **T-015** Padrão "agregados honestos": toda superfície de resumo carrega sinal de
      severidade suficiente; verde no topo NÃO coexiste com pendência no detalhe.
- [ ] **T-016** Operações destrutivas explicitadas (ex.: eliminação LGPD = irreversível +
      crypto-shredding + confirmação L3).
- [ ] **T-017** Segurança de sessão do cliente no herdado: sessão por cookie (sem token em
      `localStorage`/`sessionStorage`), CSRF, CSP restritiva, back-channel logout, encerramento
      por inatividade conforme política do tenant.
- [ ] **T-018** i18n pt-BR/en-US e revisão de terminologia PAM das telas novas (pt-BR base já feito).

## Telas herdadas — auditar/adaptar (já existem em antd)
- [ ] **T-019** Auditar as telas existentes (organizações/memberships, usuários/grupos,
      aplicações e clientes OIDC/SAML, provedores/sincronismos, MFA, papéis/permissões) contra o
      `/api/v1` e o modelo mental de PAM; ajustar navegação e remover o que não se aplica.

## Verificação
- [ ] **T-020** E2E dos fluxos privilegiados (break-glass, revisão de acesso, verificação de
      trilha) + auditoria de acessibilidade por teclado nos fluxos L3.
- [ ] **T-021** ADR-0020 ratificado; ADR-0004 marcado Superado; RFC-0005 marcado diferido.

## Gate de verificação
E2E verde nos fluxos privilegiados; teste de contrato do CI verde (console↔`/api/v1`); nenhum
endpoint "só para a UI"; nenhuma regra de autorização no frontend; navegação por teclado nos L3.

## Nota de dependência
Várias telas de PAM exigem endpoints `/api/v1` que podem ainda não estar expostos (break-glass,
revisão de acesso, verificação de trilha, saúde). Onde faltar, **o endpoint público vem antes da
tela** (I-7.6) — isso é trabalho de backend do pacote 011/capacidades, registrado por tarefa
quando surgir.
