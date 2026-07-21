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
- [ ] **T-009** Implementar `GlobalRepository` explícito, autorizado e auditado.
- [ ] **T-010** Habilitar RLS por tabela e configurar papel da aplicação sem `BYPASSRLS`.
- [ ] **T-011** Implementar contexto de tenant ativo na sessão.
- [ ] **T-012** Implementar troca de tenant com reemissão de token e auditoria.
- [ ] **T-013** Implementar fluxo de convite e vinculação de identidade existente.
- [ ] **T-014** Implementar revogação em cascata (identidade → memberships → sessões).
- [ ] **T-015** Ferramenta de inventário e deduplicação com relatório de conflito.
- [ ] **T-016** Rotina de fusão assistida (com aprovação humana obrigatória).
- [ ] **T-017** Testes de travessia entre tenants (barreira 1 e barreira 2 isoladamente).
- [ ] **T-018** Teste automatizado que rejeita query sem predicado de tenant.
- [ ] **T-019** Ensaio de migração em cópia de produção e relatório de resultado.
- [ ] **T-020** Atualizar `DIVERGENCE.md` com o escopo da divergência de modelo de dados.

## Gate de verificação
Testes de travessia verdes com RLS ligada **e** desligada (provando as duas barreiras);
ensaio de migração sem perda de fator MFA; nenhuma identidade humana duplicada no relatório
final.
