# Tasks — 002 · Identidade global e multi-tenancy B2B

- [x] **T-001** Modelar e migrar `identity` (UUIDv7, `sub` opaco, status, campos cifrados).
      *(Domínio `internal/domain/identity.go`: `Identity` com `IdentityType` (human|service) e
      `IdentityStatus` (active|suspended|deprovisioned); `NewIdentity` gera UUIDv7 (google/uuid) +
      subject opaco de 128 bits (crypto/rand, base64url, distinto do id — não vaza tempo);
      transições Suspend/Reactivate/Deprovision honram o terminal R5. Campos pessoais como
      ciphertext `[]byte` — domínio não vê PII. Migration `0002_create_identity.sql`: tabela
      CROSS-TENANT (R1, sem organization_id), `id uuid` sem default (UUIDv7 da app), CHECKs de
      type/status espelhando o domínio, `subject` UNIQUE, `email_hash` como coluna (índice único +
      login por hash ficam no T-003). Verificado em PostgreSQL 15 real: estrutura, idempotência,
      CHECKs e UNIQUE rejeitam inválidos, defaults. Teste de integração do migrator gated por
      `ARCHGUARD_TEST_DSN` (pula sem PG); domínio 100% unitário. Gate local verde.)*
- [x] **T-002** Modelar e migrar `membership` com unicidade `(identity_id, organization_id)`.
      *(Decisão do arquiteto sobre `organization_id`: `organization` ganha `id uuid` estável
      (migration `0003_organization_stable_id.sql`, DEFAULT gen_random_uuid + índice único;
      backfill por linha validado; struct XORM intocado — id lido via pgx, RFC §5). Domínio
      `internal/domain/membership.go`: `Membership` com `MembershipStatus`
      (invited|active|suspended|revoked) e máquina de estados invited→active→suspended⇄active,
      qualquer→revoked (terminal, R4); `NewMembership` (direto=active), `NewInvitedMembership`
      (invited + invitedBy); atributos de tenant como ciphertext `[]byte`. Migration
      `0004_create_membership.sql`: FKs para identity(id) e organization(id), status NOT NULL SEM
      default (força invited/active explícito), UNIQUE (identity_id, organization_id) = R3,
      índices por identity e por organization. Verificado em PG15 real: FKs, CHECK, NULL-sem-
      default e UNIQUE rejeitam inválidos; backfill de orgs pré-existentes com ids distintos.
      Gate local verde.)*
- [x] **T-004** Introduzir interface `KeyCustodian` (implementação provisória marcada como
      não suportada em produção). *(Feito ANTES do T-003 por dependência: o `email_hash` precisa
      da chave de deployment do custodiante. Porto `internal/domain/keycustodian.go`:
      `KeyCustodian.HashEmail(email) → []byte` (a chave nunca sai do custodiante) + `NormalizeEmail`
      (trim + lowercase Unicode; decisão do arquiteto: SEM canonicalização agressiva — fundir
      titulares é inaceitável num PAM). Impl. provisória `internal/adapters/keycustodian`:
      HMAC-SHA256, chave ≥ 256 bits com cópia defensiva, **marcada não-produção** (custódia/rotação
      real = pacote 010/OpenBao). Unitários: determinismo, normalização, chave distinta → hash
      distinto, rejeição de e-mail vazio e de chave fraca.)*
- [x] **T-003** Introduzir `email_hash` (HMAC) com índice único e caminho de login por hash.
      *(Migration `0005_identity_email_hash_unique.sql`: índice ÚNICO PARCIAL em
      `identity(email_hash) WHERE email_hash IS NOT NULL` — contas de serviço/deprovisionadas sem
      e-mail não colidem. `IdentityStore` pgx (`internal/adapters/postgres/identity_store.go`):
      Create + FindByEmailHash + FindByEmail (login por hash: hasheia via custodiante e busca por
      email_hash, nunca compara plaintext). Identity é cross-tenant → store sem contexto de tenant
      (distinto do T-008); UUID trafega como texto (sem depender do codec pgx). Verificado em PG15
      real: login case-insensitive acha a mesma identidade, unicidade de email_hash, not-found,
      não-colisão de contas sem e-mail. Gate local verde.
      **Achado de conformidade (I-3.3 pétreo):** os campos pessoais de 0002/0004
      (primary_email_enc, email_hash, display_name_enc, attributes_enc) estavam SEM classificação
      LGPD. Remediado na migration `0006_lgpd_classification.sql` (COMMENT ON COLUMN com categoria/
      finalidade/base legal/retenção), com teste que exige a classificação. **Follow-up:** falta o
      gate automatizado que REJEITE campo pessoal novo sem classificação — registrar no pacote 010.)*
- [x] **T-005** Migrar credenciais e fatores MFA para a identidade. *(Escopo: esquema + domínio +
      mecanismo — execução em massa é o T-019, ver decisões do arquiteto. **INV-7 codificado no
      tipo e no banco:** o legado guarda TotpSecret/RecoveryCodes em claro; o modelo novo não tem
      coluna de segredo reversível. Porto `SecretStore` (`internal/domain/secretstore.go`,
      Put/Get por referência) com impl provisória sobre o keystore selado (`internal/adapters/
      secretstore`, não-produção; prod=OpenBao). Domínio `credential.go`: `FactorType`
      (password|totp|webauthn|recovery_code) + `AAL` (ADR-0010); construtores tornam impossível um
      TOTP segurar o seed (só `SecretRef`); `WellFormed()` = a forma INV-7 exata. Migration
      `0007_create_credential.sql`: tabela cross-tenant, CHECK `credential_shape` amarra cada tipo
      ao seu material (segredo reversível só como secret_ref), 1 senha/1 TOTP por identidade,
      classificação LGPD inline. `CredentialStore` pgx (Create com guarda WellFormed + ListByIdentity).
      Mecanismo `internal/credmigration`: move seed TOTP ao cofre, hasheia recovery codes (SHA-256),
      **senha em claro do legado NÃO é carregada — força reset** (INV-1/INV-7), carrega WebAuthn
      público. Verificado em PG15 real: 4 tipos persistem, guarda de app e CHECK do banco rejeitam
      forma INV-7-inválida, unicidade de senha; unitários provam que o seed vai ao cofre e nunca à
      credencial. Gate local verde.)*
- [x] **T-006** Repontar papéis e permissões para `membership_id`. *(Decisões do arquiteto:
      estender `role` legada com UUID estável — como organization — e permissões Casbin superadas
      pelo OpenFGA (pacote 007), fora do escopo. Migration `0008_role_stable_id.sql`: `role.id uuid`
      + índice único (mesma mecânica de organization). `0009_create_role_assignment.sql`: tabela
      `role_assignment` por MEMBERSHIP (R2), FKs para organization(id)/role(id)/membership(id),
      `organization_id NOT NULL` (R1, pronta p/ RLS T-010), UNIQUE (role_id, membership_id) —
      substitui o `Role.Users[]` denormalizado. Domínio `role_assignment.go`: `NewRoleAssignment`
      referencia membership, **sem campo identity** (R2 no tipo). `RoleAssignmentStore` pgx
      (Create + ListByMembership). Mecanismo `internal/rolemigration`: resolve `Role.Users[]`
      (`org/user`) → membership via porto `MembershipResolver` (real ligado no T-019), dedup por
      membership, não-resolvidos reportados. Verificado em PG15 real: Create/List, FKs, UNIQUE R2.
      Fronteira registrada: motor de permissões → OpenFGA (007); grupos/papéis aninhados adiados.)*
- [x] **T-007** Backfill de `organization_id` nas tabelas de domínio. *(Decisões do arquiteto:
      núcleo PAM curado + inventário completo; linhas globais cross-tenant com org_id NULL.
      **Inventário** `docs/upstream/TENANT_INVENTORY.md` (o artefato que o R1 referencia): classifica
      cada tabela como tenant-scoped / cross-tenant (identity, credential, organization + linhas
      admin/built-in/IsShared) / fora de escopo PAM. Migration `0010_backfill_organization_id.sql`:
      bloco `DO` dinâmico e idempotente que, para as ~16 tabelas do núcleo (user, group, role,
      permission, token, session, invitation, resource, syncer, webhook, application, provider, cert,
      adapter/enforcer/model), adiciona `organization_id uuid` (nullable), popula via `owner →
      organization.id`, cria FK + índice; pula tabela ausente. **Linhas globais (owner=admin) ficam
      NULL** (R1). NOT NULL fica para o T-010, por tabela, só onde 100% mapeia. Verificado em PG15
      real: tenant→org.id, global→NULL, FK+índice; teste re-executa a 0010 real do FS embutido
      (idempotente, order-independent). Gate local verde.)*
- [x] **T-008** Implementar repositório com contexto de tenant obrigatório (barreira 1).
      *(Decisões do arquiteto: SET LOCAL por transação; fundamento + role_assignment tenant-scoped.
      Domínio `tenant.go`: `TenantScope` (recusa `uuid.Nil` = ErrNoTenant — não há repositório sem
      tenant) + constante `RLSOrgSettingName` = `app.current_organization` (contrato com a RLS do
      T-010, travado por teste). Adapter `tenant.go`: `TenantRepository` (construtor exige scope) +
      `WithTenantTx` que abre uma transação (RFC §5) e faz `set_config(name, org, true)` — SET LOCAL,
      escopo de tx, nunca vaza no pool; é o gancho que a RLS lê. `TenantTx` componível (vários stores,
      uma transação). `RoleAssignmentStore` migrado para tenant-scoped: predicado explícito
      `AND organization_id = $scope` nas leituras (Barreira 1, vale mesmo com RLS off) e recusa de
      escrita cross-tenant (ErrCrossTenantWrite). Verificado em PG15 real: **travessia de leitura
      isolada** (repo de A não vê dados de B por membership de B), escrita cross-tenant recusada,
      parâmetro de sessão fixado no valor certo, UNIQUE R2. Gate local verde.)*
- [x] **T-009** Implementar `GlobalRepository` explícito, autorizado e auditado. *(Tipo DISTINTO do
      tenant-scoped — cruzar tenant nunca acontece por acaso. Portos de domínio `globalaccess.go`:
      `GlobalAuthorizer` (INV-6: erro = negação) e `AccessAuditor` (I-5.4: falha de auditoria =
      negação), com `GlobalAccess{Principal, Reason}` obrigatórios. `GlobalRepository.WithGlobalTx`:
      valida → autoriza → audita → só então roda a tx com o flag `app.global_read=on` (SET LOCAL)
      que a RLS honra. Selos provisórios `internal/adapters/globalaccess`: `ProfileAuthorizer`
      (permite só em dev, nega em pilot/prod até o OpenFGA do 007 — fail-closed) e `MemoryAuditor`
      (dev-only, não-durável; trilho real = 003). Demo `MembershipStore.ListByIdentity` (cross-tenant,
      "meus tenants"). Verificado em PG15 real: lê 2 tenants + auditado; negado sem autz; negado
      quando auditoria falha; recusa acesso sem motivo.)*
- [x] **T-010** Habilitar RLS por tabela e configurar papel da aplicação sem `BYPASSRLS`. *(Barreira 2.
      Migration `0011_enable_rls.sql`: RLS + FORCE em `membership` e `role_assignment` (só tabelas
      NOVAS — ligar nas legadas quebraria o XORM que consulta sem o parâmetro; ativação incremental,
      RFC §6). Policy: LÊ se `organization_id = app.current_organization` OU `app.global_read=on`;
      ESCREVE (WITH CHECK) só se `= app.current_organization` — sem escrita global. `NULLIF(...,'')`
      protege parâmetro ausente. `roles.sql`: `archguard_app`/`readonly` com **NOBYPASSRLS explícito**
      (+ ALTER idempotente). Verificado em PG15 real como papel NÃO-superusuário: tenant A não vê
      vínculo de B mesmo sem predicado de aplicação (Barreira 2 isolada), leitura global torna B
      visível, WITH CHECK bloqueia escrita cross-tenant. **As duas barreiras agora provadas
      independentemente** (T-008 com RLS off, T-010 com RLS on). Gate local verde.)*
- [x] **T-011** Implementar contexto de tenant ativo na sessão. *(Decisões do arquiteto:
      sessão pendente É persistida (linha `pending_selection` sem tenant) e RLS da tabela
      DIFERIDA para o T-012 — a escrita do login não tem contexto de tenant e a policy do
      T-010 não tem escrita global. Domínio `authsession.go`: `AuthSession` carrega
      identity_id + membership_id ATIVO; "exatamente um tenant ativo" é ESTRUTURAL (um único
      campo). `NewAuthSession(identity, provenAAL, memberships)`: 0 membership ativo →
      negação; 1 → auto-seleção (nasce active); >1 → nasce `pending_selection` e
      `ActiveTenant()` retorna `ErrTenantSelectionRequired` — o cenário "Múltiplos
      memberships no login" codificado no tipo (token só com sessão active, emissão = T-012).
      `SelectTenant` só de pending, só membership ativo E da própria identidade (fail-closed
      p/ membership alheio); re-seleção de sessão active é recusada (troca = T-012, com
      reemissão + auditoria); revoked terminal, contexto preservado p/ trilha. `ProvenAAL`
      (ADR-0010) registrado p/ o step-up do T-012/005. Migration `0012_create_auth_session.sql`:
      `auth_session` (nome novo — `session` é a legada Beego; ponte = T-014), CHECK
      `auth_session_tenant_shape` (active ⇒ tenant NOT NULL; pending ⇒ NULL) e FK COMPOSTA
      (membership_id, identity_id, organization_id) → membership: o banco recusa tenant ativo
      que não seja membership DESTA identidade NESTA org. Sem campo pessoal (IP/UA = auditoria,
      RFC-0003). Stores: `IdentitySessionStore` (novo `domain.IdentityScope`, Barreira 1 no
      eixo identidade — sem construtor sem identidade; Create/Get/SaveSelection/Revoke com
      predicado identity_id; SaveSelection só de pending, anti-corrida) e `TenantSessionStore`
      (via TenantTx, `ListActive` da org — insumo do T-014). `TENANT_INVENTORY.md` atualizado.
      Verificado em PG15 real: auto-seleção, pendente→seleção explícita persistida, CHECK e FK
      composta recusam atalhos via SQL cru, isolamento por identidade e por tenant, revogação
      idempotente. Gate local verde; `make test` completo tem 7 pacotes LEGADOS quebrando
      pré-existentes em HEAD (testes env-dependent do upstream — ver nota da sessão).)*
- [x] **T-012** Implementar troca de tenant com reemissão de token e auditoria. *(Decisões do
      arquiteto: reemissão materializada como `token_generation` na sessão (JWT real = 006) e
      RLS da `auth_session` ligada AGORA com contexto de identidade. Domínio `tenantswitch.go`:
      `AAL.AtLeast` (aal1<aal2<aal3, fail-closed p/ nível indefinido);
      `AuthSession.SwitchTenant(dest, required)` — só de sessão active, destino ativo E da
      própria identidade, mesmo-tenant recusado (`ErrSameTenant`, sem evento), destino mais
      forte que o comprovado ⇒ `ErrStepUpRequired` = NEGAÇÃO (step-up real = 005), política
      indecidível ⇒ `ErrDestinationPolicyUnavailable` (INV-6); sucesso move o contexto,
      INCREMENTA a geração e produz `TenantSwitchEvent` (from/to validados). `TokenContext`
      (sub/org/mid/sid/aal/geração — RFC-0006 §3) só deriva de sessão ATIVA: token nunca
      carrega dois tenants; token pré-troca tem geração antiga e falha por construção. Portos
      `TenantAuthPolicy` (real = 005) e `SessionAuditor` (real = 003); selos provisórios em
      `internal/adapters/tenantswitch`: `ProfilePolicy` (dev=AAL1, pilot/prod=AAL3 — só
      sobre-exige) e `MemorySwitchAuditor` (dev-only, valida evento). Migration 0013:
      `token_generation` (CHECK ≥1) + RLS+FORCE na `auth_session` — lê identidade própria
      (`app.current_identity`, novo `domain.RLSIdentitySettingName`) OU org corrente OU
      global_read; escreve identidade própria OU org corrente. `IdentityRepository`/
      `WithIdentityTx` (SET LOCAL, espelho do TenantRepository); `IdentitySessionStore`
      refatorado para IdentityTx + `SaveSwitch` OTIMISTA (origem+geração esperadas; corrida ⇒
      `ErrSwitchConflict`, geração estritamente serial). Orquestração `TenantSwitcher.Switch`:
      política ANTES da tx (RFC-0004 §4) → transição → persistência → `RecordTenantSwitch`
      DENTRO da tx — falha de auditoria ⇒ ROLLBACK da troca (I-5.4, provado em teste).
      Verificado em PG15 real: troca persiste destino+geração 2 e audita from→to; step-up
      nega sem alterar nada; política com erro nega; auditoria falhando desfaz a troca;
      conflito otimista; RLS da auth_session como papel não-superusuário (pendente só pelo
      eixo identidade, ativa pelo eixo tenant, WITH CHECK barra escrita alheia). Gate verde.)*
- [x] **T-013** Implementar fluxo de convite e vinculação de identidade existente. *(Decisões do
      arquiteto: (1) e-mail SEM identidade correspondente fica FORA do escopo — `Inviter` devolve
      `ErrUnknownInviteEmail` sem criar nada silenciosamente; criação de identidade via convite
      (onboarding/cifra por titular) chega com 008/009. (2) **R3 estrita**: par com membership
      REVOGADO recusa readmissão (`ErrPreviouslyRevoked`) — o UNIQUE da R3 bloqueia segunda linha
      e o revogado permanece na trilha; **QUESTÃO REGISTRADA para emenda do RFC-0002**: readmitir
      (prestador que volta) exigiria índice único parcial `WHERE status != 'revoked'`, decisão
      normativa pendente. `TenantMembershipStore` (sobre TenantTx, Barreira 1): `Create` classifica
      colisão R3 (`ErrAlreadyMember`/`ErrPreviouslyRevoked`, corrida coberta pelo 23505), `Get`
      org-scoped, `SaveActivation` só de invited (anti-corrida, carimba `activated_at`).
      `Inviter` (padrão TenantSwitcher): `InviteByEmail` em UMA transação — busca por HASH via
      `KeyCustodian` (plaintext nunca no banco), identidade não-ativa recusada
      (`ErrIdentityNotInvitable`, fail-closed), `NewInvitedMembership` persistido no tenant
      convidante com `invited_by`; `Accept` só pela identidade convidada (`ErrNotInviteOwner`) +
      transição de domínio invited→active. Sem porto de auditoria aqui: mutação administrativa
      entra na trilha no pacote 003. Verificado em PG15 real: convite vincula identidade
      EXISTENTE sem criar identidade (contagem provada) e preserva o membership de A (2 vínculos,
      1 conjunto de credenciais); case-insensitive como o login; e-mail desconhecido sem efeito
      colateral; colisões R3 classificadas; aceite ativa/carimba e re-aceite recusa; guardas
      cross-tenant de leitura e escrita. Gate verde.)*
- [x] **T-014** Implementar revogação em cascata (identidade → memberships → sessões). *(Decisões
      do arquiteto: (1) suspensão da identidade SUSPENDE memberships (recuperável;
      terminal é reservado ao deprovisionamento R4/R5) e revoga TODAS as sessões (sessão é
      sempre terminal); (2) migration 0014 emenda a policy RLS de `membership` com o eixo de
      identidade (`identity_id = app.current_identity`, leitura E escrita) — a cascata
      cross-tenant roda em UMA transação (RFC-0002 §5, sem cascata parcial), mesmo modelo da
      auth_session/0013. `MembershipRevoker.RevokeMembership` (WithTenantTx): membership →
      revoked + revoked_at e `RevokeByMembership` encerra as sessões DAQUELE membership —
      predicado org garante que sessões de outros tenants ficam intactas (o AND da spec);
      idempotente ponta a ponta; membership de outro tenant inalcançável. `IdentityLifecycle`
      (WithIdentityTx): `Suspend` (identidade suspended + `SuspendAllActive` + `RevokeAll`
      incluindo sessão pendente) e `Deprovision` (terminal R5 + `RevokeAllNonRevoked` + sessões;
      re-executar = 0 movimentos; Suspend pós-deprovisionamento recusado). `IdentityStore` ganhou
      `Get`/`SaveStatus`; `IdentityMembershipStore` (eixo identidade, Barreira 1). Fora do
      escopo, registrado no TENANT_INVENTORY: ponte com a `session` legada Beego — sem vínculo
      identity↔user no esquema ainda (T-019/005/006). Auditoria da revogação → trilha do 003.
      Verificado em PG15 real: cascata por membership só no tenant dele (B intacto), cascatas de
      suspensão (2 memberships suspensos, 3 sessões revogadas) e deprovisionamento (terminal),
      RLS da 0014 como papel não-superusuário (eixo identidade só alcança as próprias linhas,
      UPDATE alheio atinge 0 linhas, eixo tenant preservado). Gate verde.)*
- [x] **T-015** Ferramenta de inventário e deduplicação com relatório de conflito. *(Pacote
      `internal/identdedup` — RFC-0002 §6 passos 1–2, padrão dos mecanismos: função PURA sobre
      primitivos legados (sem XORM/banco), extração em massa fica no T-019. `BuildInventory`
      hasheia cada e-mail via `KeyCustodian` (case-insensitive pela normalização) e classifica:
      1:1 (e-mail único), **candidata a fusão** (mesmo hash em orgs distintas → 1 identidade +
      N memberships, SÓ PROPOSTA — execução exige aprovação humana no T-016), sem e-mail
      (identidade própria, sem hash — índice único parcial permite) e **conflitos p/ revisão
      humana**: `same_org_duplicate` (mesmo e-mail 2× na MESMA org — fusão violaria R3),
      `mixed_types` (e-mail compartilhado por humana e serviço) e `unhashable_email`. Grupo
      conflitado NUNCA vaza para as propostas. Flags de fator (senha/totp/webauthn) viajam por
      conta — insumo do "sem perda de fator MFA" do gate. Determinística (mesma base em qualquer
      ordem = mesmo inventário; o ensaio T-019 diffa execuções). `Render` emite o relatório
      pt-BR **sem e-mail em claro** (minimização: owner/name + prefixo do hash), provado por
      teste. Gate verde.)*
- [x] **T-016** Rotina de fusão assistida (com aprovação humana obrigatória). *(Pacote
      `internal/identfusion`, padrão dos mecanismos — persistência/execução em massa = T-019.
      **Aprovação humana é ESTRUTURAL**: `Fuse` não roda sem `Approval{ApprovedBy, hash do
      grupo, conta primária}` — aprovador vazio = `ErrFusionNotApproved`; aprovação AMARRADA ao
      grupo pelo email_hash (aprovação do grupo X jamais autoriza Y = `ErrApprovalMismatch`);
      a conta PRIMÁRIA (eleita pelo humano, tem de estar no grupo) decide os fatores de slot
      único (senha/TOTP/recovery — o esquema permite 1 de cada por identidade). Defesa em
      profundidade: classes de conflito do T-015 REVALIDADAS na execução (`ErrGroupNotFusable`:
      <2 contas, mesma org=R3, conta de serviço). Saída: identidade humana única carregando o
      email_hash do grupo (plaintext nunca necessário), 1 membership ATIVO por org (porto
      `OrganizationResolver`, real=pgx no T-019), credenciais via `credmigration` (seed TOTP ao
      cofre, senha em claro força reset INV-1/INV-7); contas não-primárias contribuem SÓ
      WebAuthn (multi-slot) e cada fator de slot único não carregado vai para `DroppedFactors`
      — nada descartado em silêncio, e a identidade fundida não perde TIPO de fator (gate do
      pacote). Unitários cobrem todas as recusas e o caminho feliz. Gate verde.)*
- [x] **T-017** Testes de travessia entre tenants (barreira 1 e barreira 2 isoladamente).
      *(Suíte dedicada `test/invariants/inv5_traversal_test.go` — INV-5/I-6.3 na suíte que
      QUEBRA O BUILD (`make invariants`), como o RFC-0002 §4 exige. Fixture: identidade com
      membership+papel+sessão em A e em B. **Barreira 1 ISOLADA** (= RLS desligada): roda como
      superusuário, que ignora RLS — todo isolamento observado é dos predicados/guardas;
      leitura de B por stores escopados em A vazia/not-found nas 3 tabelas novas, escrita
      cross-tenant recusada, escopo nulo recusado. **Barreira 2 ISOLADA** (= aplicação
      contornada): papel NOBYPASSRLS executa SQL cru SEM predicado de aplicação — contexto de A
      vê A e NÃO vê B (membership, role_assignment, auth_session), sem contexto não vê nada,
      WITH CHECK barra INSERT de linha de B sob contexto de A. Bônus: `TestINV5RLSStaysEnabled`
      trava ENABLE+FORCE nas 3 tabelas (migration futura que desligue RLS quebra o build).
      Gated por `ARCHGUARD_TEST_DSN` (o CI deve prover PG para o gate valer — nota p/ pacote
      001/CI diferido). Gate verde.)*
- [x] **T-018** Teste automatizado que rejeita query sem predicado de tenant. *(Detector de
      análise estática `test/invariants/inv5_query_test.go` — o cenário "Query sem predicado de
      tenant" na suíte que quebra o build, SEM precisar de banco (roda sempre, mesmo sem DSN).
      Varre os fontes Go de `internal/` (o mundo pgx = código novo; legadas entram quando o
      acesso migrar do XORM), extrai literais de string via AST (`go/parser`) e, para query que
      toque tabela guardada (`membership`, `role_assignment`, `auth_session` — manter em sincronia
      com o TENANT_INVENTORY e com o trava-RLS do T-017): SELECT/UPDATE/DELETE exigem WHERE com
      `organization_id` OU `identity_id` (eixo sancionado pelas policies 0013/0014); INSERT exige
      a coluna de escopo. Limitação declarada: cobre literais (o padrão da casa de SQL explícito);
      concatenação dinâmica não é aceita em revisão. Self-tests: fixture
      `testdata/inv5/` com 3 violações injetadas (uma por tabela) acusadas exatamente, e bateria
      anti-falso-positivo (mensagens de erro com nome de tabela, queries escopadas, pg_class).
      Código atual de `internal/`: 0 violações. Gate verde.)*
- [x] **T-019** Ensaio de migração em cópia de produção e relatório de resultado. *(Pacote
      `internal/migrehearsal` — o pipeline COMPLETO do RFC-0002 §6 sobre uma cópia descartável:
      extração do legado por SQL primitivo (user/role, sem XORM; recovery_codes/webauthn como
      JSON; e-mail em claro só vive até o hash) → inventário T-015 → fusão T-016 SÓ com
      aprovação humana (grupo sem aprovação vai para `PendingApproval`, NÃO migra) → conflitos
      reportados e pulados → credenciais T-005 (seed TOTP ao cofre ANTES da tx, RFC-0004 §4;
      senha em claro = reset forçado) → papéis T-006 com resolvedores REAIS (`orgResolver` pgx
      por nome→uuid; memberships criados alimentam o `MembershipResolver`; não-resolvidos
      reportados). Persistência: UMA transação por grupo via `WithIdentityTx` (o eixo de
      identidade da 0014 permite os N memberships da identidade fundida numa tx);
      role_assignments por org via `WithTenantTx`. **Relatório com a validação do gate**:
      `checkFactorPreservation` acusa perda de TIPO de fator MFA (ex.: aprovação elegendo
      primária sem TOTP quando outra conta o tem → `FactorLoss` e `Validate()` FALHA — provado
      por teste dedicado) e contagem de identidades humanas duplicadas por email_hash (= 0);
      `Render` pt-BR sem dado pessoal em claro (provado). E2E verificado em PG15 real: alice
      fundida (1 identidade, 2 memberships, senha+TOTP+recovery da primária, WebAuthn da
      secundária, descartes reportados), bob com reset forçado, conta de serviço sem e-mail,
      carol em conflito R3 não migrada, dave pendente de aprovação não migrado, papel vinculado
      ao membership com fantasma reportado, seed TOTP no cofre e NUNCA no banco. Gate verde.)*
- [x] **T-020** Atualizar `DIVERGENCE.md` com o escopo da divergência de modelo de dados.
      *(Linha 2026-07-21 no `docs/upstream/DIVERGENCE.md`: a maior divergência estrutural do
      fork (prevista no ADR-0006) — tabelas novas (identity/membership/credential/
      role_assignment/auth_session), extensões das legadas (id uuid em organization/role,
      organization_id em ~16 tabelas), duas barreiras de isolamento com os três contextos de
      sessão de banco, mecanismos de migração e impacto de triagem: o modelo user=identidade-
      por-org do upstream está SUPERADO — commits que mudem semântica de user/organization/
      role/session/invitation exigem revisão manual. Prefixos refletidos em
      `tools/upstreamwatch/classify.go` (role.go, session.go, invitation) como o próprio
      DIVERGENCE.md pede. Gate verde.)*

## Gate de verificação
Testes de travessia verdes com RLS ligada **e** desligada (provando as duas barreiras);
ensaio de migração sem perda de fator MFA; nenhuma identidade humana duplicada no relatório
final.
