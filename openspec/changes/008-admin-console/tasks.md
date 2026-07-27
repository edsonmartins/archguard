# Tasks — 008 · ArchGuard Console (evolução do console herdado)

> Base normativa: **ADR-0020** (ratificado 2026-07-26, supersede o ADR-0004). Este re-escopo troca
> "reescrever o console" por "**evoluir o console herdado**" (CRA + antd, já rebrandizado/
> tematizado), construindo nele as telas de PAM que faltam, contra o `/api/v1` (pacote 011).
> (Greenfield anterior no histórico git.)

## Fundação e travas (mantêm os invariantes do ADR-0004)
- [x] **T-001** Camada de API tipada do console para o `/api/v1` (plano de controle, pacote 011):
      módulo dedicado `web/src/backend/ControlPlaneBackend.js` (helper `cpRequest` + funções
      tipadas por JSDoc: session/tenants/memberships/grants/audit-timeline/audit-verify/
      access-effective/health/stepup/factors; sessão por cookie; distinção denied×error).
      Ponto único de acesso ao `/api/v1` (base da trava T-003). eslint verde.
- [x] **T-002** Teste de contrato no CI (`test/contract/console_api_contract_test.go`): estático,
      sem DB — toda rota `/api/v1` chamada pelo console (`cpRequest` em ControlPlaneBackend.js)
      DEVE estar montada no backend (`RegisterAPIHandler` em internal/boot/mounts.go). Falha o CI
      em *drift* (console chama rota não montada; I-7.6). `go test`/`go vet` verdes.
- [x] **T-003** Trava do ponto único: teste estático (`console_api_boundary_test.go`) proíbe
      referência crua a `/api/v1` fora de `ControlPlaneBackend.js` — todo acesso passa pela camada
      tipada. As demais garantias (nenhum endpoint "só para a UI" — I-7.6; nenhuma authz no
      frontend) são de revisão de PR, ancoradas no gate de assurance/RequireAdmin do backend e na
      spec (cenário "Elemento oculto"). `go test` verde.

## Contexto de operação (cross-cutting)
- [ ] **T-004** Seletor de tenant permanente no cabeçalho com distinção visual inequívoca do
      tenant ativo; troca reemite token e dispara step-up se a política do destino for mais
      restritiva.
      - [x] **Parte A (backend)** — `POST /api/v1/session/tenant` (`internal/http/session_switch.go`
        + wrapper `internal/boot/tenant_switch.go`, montado em mounts.go, op `session.switch_tenant`
        L1): resolve o membership ATIVO do próprio chamador (nunca do request, INV-1) e delega ao
        `postgres.TenantSwitcher` (política do destino → step-up denial; TokenGeneration++; auditoria
        atômica). `ErrStepUpRequired` → 401 RFC 9470; negações → 403/409; fail-closed (política/
        auditoria) → 503. `switchTenant()` no ControlPlaneBackend. Testes de todos os caminhos +
        http/boot/contract/invariantes verdes.
      - [x] **Parte B (frontend)** — `web/src/common/select/TenantSelect.js`: selo verde proeminente
        do tenant ativo (distinção inequívoca); multi-membership → dropdown de troca. Troca chama
        `switchTenant()` → 200 recarrega o contexto; 401 → aviso de step-up (tratamento mínimo;
        transparente é a T-005); 403 → não-membro. Ligado no cabeçalho do ManagementPage (coexiste
        com o OrganizationSelect herdado). `/api/v1/tenants` enriquecido com `display_name`
        (namer `OrgDisplayNamer`, fallback ao UUID). i18n en+pt. Ponte de login já ligada
        (auth.go→bridgeDomainSession→BridgeLogin), então o admin logado resolve o `/api/v1`.
        Build local + VPS + testes verdes.
- [x] **T-005** Interceptor global de step-up: no `cpRequest` (ControlPlaneBackend), um 401 RFC 9470
      (`WWW-Authenticate: insufficient_user_authentication`) chama um handler registrado por
      `web/src/common/StepUpModal.js` (montado no App), que conduz o desafio TOTP (`/stepup/totp`) e,
      no sucesso, **repete a operação original uma vez** — o formulário do chamador é preservado (ele
      só aguarda a promessa); cancelar rejeita e mantém o form. Distingue step-up de 401 de sessão
      ausente; nunca recursa no próprio `/stepup/totp`. Fator: TOTP (o exposto no `/api/v1`); WebAuthn
      é refinamento posterior. i18n en+pt. Build local + contrato verdes.

## Telas críticas de PAM (o que o herdado não tem)
- [x] **T-006** Concessões vigentes (privileged grants) com contagem regressiva e revogação.
      - [x] **Parte A (lista)** — `web/src/GrantsPage.js`: tabela das concessões vigentes do tenant
        ativo (GET `/api/v1/grants`, lê a org da sessão) com **contagem regressiva ao vivo** até
        `expires_at` (verde / laranja <5min / vermelho expirado), alvo/origem/status. Rota `/grants`
        + grupo de menu "Acesso Privilegiado" (ícone Tabler `key`). i18n en+pt. Fail-closed (sem
        contexto/negação → vazio). Build local verde.
      - [x] **Parte B (revogação)** — `POST /api/v1/grants/revoke` (`internal/http/grants_write.go` +
        wrapper `internal/boot/grant_revoke.go`, montado em mounts.go, op `grant.revoke` **L3**):
        delega ao `PrivilegedAccessService.Revoke` (revoga a concessão + cascateia a revogação das
        sessões derivadas + auditoria `privileged.grant.revoke`, atômico — I-5.4). Grant escopado
        pela RLS do tenant da sessão (INV-5: outro org → not-found); ator/org da sessão, nunca do
        request (INV-1). `ErrGrantNotFound`→404, não-ativa→409, fail-closed→500. `revokeGrant()` no
        ControlPlaneBackend + botão **Revogar** no GrantsPage (só em `active`, `Popconfirm` que
        explicita a consequência destrutiva — cenário "Operações destrutivas explicitadas"; L3 via
        step-up transparente da T-005). i18n en+pt. Build/boot/http/contrato/invariantes + yarn build
        verdes.
- [x] **T-007** Solicitação de break-glass com justificativa e incidente. Fundação + request.
      Fundação: adaptador `Notifier` concreto (`internal/adapters/notification`) sobre os
      provedores de notificação do tenant (fail-closed: `Available`=há canal; `Notify`=entrega
      ou falha) + `BreakglassPolicy` por perfil (prod exige `DefaultBreakglassApprovals`=2; dev=1).
      `POST /api/v1/breakglass/request` (`internal/http/breakglass.go` + wrapper
      `internal/boot/breakglass.go`, op `breakglass.request` **L3** + RequireAdmin; `DeniesL3()`
      bloqueia dev): delega ao `BreakglassOrchestrator` (alerta em tempo real ANTES da concessão;
      grant+auditoria atômicos). Sujeito = membership da sessão (INV-1); org da sessão (INV-5).
      `ErrNoNotificationChannel`→**503** fail-closed, validação de domínio→422. `requestBreakglass()`
      no ControlPlaneBackend + `BreakglassRequestPage` (alvo opaco, justificativa/incidente
      obrigatórios, janela; Alert explicita ser acesso de emergência auditado; L3 via step-up T-005).
      Rota `/breakglass/request` no grupo "Acesso Privilegiado". i18n en+pt. Build/boot/http/
      contrato/invariantes + yarn build verdes.
- [x] **T-008** Fila de aprovação de break-glass (separação de deveres; sem autoaprovação).
      Precedida da fundação **T-005b (step-up WebAuthn, 4 fases)**: sem ela nenhum grant chegava a
      `awaiting_approval` (o pipeline exige phishing-resistant em L3 e o TOTP só chega a AAL2).
      `GET /api/v1/breakglass/pending` (op `breakglass.pending` L1, `ListAwaitingApproval` novo no
      grant store/reader — expõe justificativa+incidente, que o aprovador precisa) + `POST
      /api/v1/breakglass/approve` (op `breakglass.approve` **L3**, `internal/http/breakglass_approve.go`
      + wrapper `breakglassApprover` no boot → `PrivilegedAccessService.Approve`). **Separação de
      deveres imposta pelo DOMÍNIO**: solicitante não aprova (`ErrSelfApproval`→403), aprovadores
      DISTINTOS (`ErrGrantDuplicateApproval`→409), só de `awaiting_approval` (→409); quórum atingido
      ⇒ ativa, atômico com a auditoria. Aprovador = membership da sessão (INV-1). `BreakglassQueuePage`
      (tabela com solicitante/alvo/justificativa/incidente; botão Aprovar via step-up transparente;
      o botão NÃO é o controle — a API nega). i18n en+pt. Build/boot/http/contrato/invariantes +
      yarn build verdes.
- [x] **T-009** Timeline de auditoria com filtros + **indicador de integridade da cadeia sempre
      visível**, com divergência em destaque máximo; acionamento da verificação (L3).
      `AuditPage.js`: banner de integridade SEMPRE visível no topo (neutro "não verificada" / verde
      "íntegra — N eventos, M selos" / **VERMELHO banner** em divergência com seq/tipo/detalhe) +
      botão "Verificar cadeia" (op `audit.verify` L3 → step-up WebAuthn transparente; divergência =
      409 no `ControlPlaneError.body`). Tabela do timeline (`/audit/timeline`, org da sessão) com
      colunas quando/ação/desfecho(cor)/ator/alvo/motivo/pcid, filtro client-side + seletor de
      limite. **Correção de segurança junto:** `/audit/verify` passou a ler a org da **sessão**
      (era do query — admin podia verificar outro tenant, BOLA cross-tenant), consistente com o
      timeline (INV-5); teste do handler atualizado. `getAuditTimeline`/`verifyAuditChain`
      simplificados. Rota `/audit` + grupo de menu "Auditoria". i18n en+pt. Build/http/contrato/
      invariantes + yarn build verdes.
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
      - [x] **Fase A (smoke L1) — Cypress (sem dep nova, ADR-0002):** `web/cypress/e2e/archguard/
        pam_smoke.cy.js` (login de API `cpLogin` → navega grants/audit/breakglass-queue/breakglass-
        request; afirma render + `/api/v1` 200 + banner de integridade sempre visível). Config
        dedicada `cypress.archguard.config.js` (baseUrl `ARCHGUARD_E2E_URL`, default :8000). Harness
        `deploy/e2e/docker-compose.e2e.yml` (Postgres efêmero + imagem do fork em perfil DEV —
        keystore local, SEM OpenBao; semeia admin/123) + alvos `make e2e`/`e2e-up`/`e2e-down`.
        **NÃO validado end-to-end pelo autor (sem Docker na máquina); sintaxe/YAML/eslint verdes —
        o run é do usuário/CI.** Não roda no CI ainda (Fase C).
      - [ ] **Fase B (fluxos L3):** stack completa (perfil pilot + OpenBao) + autenticador WebAuthn
        virtual (CDP `WebAuthn.addVirtualAuthenticator`); break-glass request→step-up→fila→aprovar
        (separação de deveres), revogar, verificar cadeia (íntegra e divergência com fixture).
      - [ ] **Fase C (CI):** job de E2E no `ci.yml` que sobe o compose e roda as specs.
- [ ] **T-021** ADR-0020 ratificado; ADR-0004 marcado Superado; RFC-0005 marcado diferido.

## Gate de verificação
E2E verde nos fluxos privilegiados; teste de contrato do CI verde (console↔`/api/v1`); nenhum
endpoint "só para a UI"; nenhuma regra de autorização no frontend; navegação por teclado nos L3.

## Nota de dependência
Várias telas de PAM exigem endpoints `/api/v1` que podem ainda não estar expostos (break-glass,
revisão de acesso, verificação de trilha, saúde). Onde faltar, **o endpoint público vem antes da
tela** (I-7.6) — isso é trabalho de backend do pacote 011/capacidades, registrado por tarefa
quando surgir.
