# RFC-0002 — Modelo de dados de identidade e multi-tenancy

- **Status:** Proposto
- **Data:** 2026-07-19
- **ADRs relacionados:** ADR-0006, ADR-0009, ADR-0014, ADR-0016

## 1. Objetivo

Especificar o modelo de dados que sustenta identidade global única com pertencimento a
múltiplas organizações, isolamento de tenant por construção e conformidade com o regime de
dados pessoais.

## 2. Entidades centrais

### 2.1 `identity` (identidade global)
Representa a **pessoa** ou conta de serviço, única no deployment.

| Campo | Notas |
|---|---|
| `id` | UUIDv7 (ordenável no tempo) |
| `subject` | Identificador opaco e estável exposto como `sub` no token. **Nunca** e-mail |
| `type` | `human` \| `service` |
| `status` | `active` \| `suspended` \| `deprovisioned` |
| `primary_email_enc` | Cifrado com chave por titular (ADR-0014) |
| `email_hash` | HMAC com chave de deployment — permite busca/unicidade sem armazenar em claro |
| `display_name_enc` | Cifrado |
| `created_at`, `updated_at` | |

Credenciais e fatores MFA pertencem à identidade, **não** ao membership.

### 2.2 `organization` (tenant)
Fronteira de isolamento. Contém política de MFA, política de break-glass (N aprovadores),
política de retenção, domínios verificados e configuração de federação.

### 2.3 `membership` (relação usuário↔organização)
Entidade explícita — o núcleo da decisão do ADR-0006.

| Campo | Notas |
|---|---|
| `id` | UUIDv7 |
| `identity_id`, `organization_id` | Chave natural única (par) |
| `status` | `invited` \| `active` \| `suspended` \| `revoked` |
| `attributes_enc` | Atributos **específicos do tenant** (matrícula, centro de custo). Nunca compartilhados entre tenants |
| `invited_by`, `activated_at`, `revoked_at` | Trilha de ciclo de vida |

### 2.4 Demais entidades
`credential` (por identidade: senha, WebAuthn, TOTP, com metadados de fator e AAL),
`group` e `group_member` (por organização, com hierarquia), `role` e `role_assignment`
(sempre por membership), `application` / `oauth_client` (por organização),
`identity_provider` e `directory_connector` (por organização), `session`,
`privileged_grant` (concessões temporárias e break-glass), `audit_event` (RFC-0003).

## 3. Regras de integridade

- **R1** Toda tabela de domínio possui `organization_id` **NOT NULL** — exceto `identity`,
  `credential` e as tabelas de configuração global, que são explicitamente listadas como
  *cross-tenant* no inventário.
- **R2** Papéis e permissões referenciam `membership_id`, **jamais** `identity_id`
  diretamente. Não existe papel global de tenant atribuído à identidade.
- **R3** Unicidade de `(identity_id, organization_id)` em `membership`.
- **R4** Revogar a identidade cascateia para todos os memberships e sessões.
- **R5** Deleção física de identidade não existe: o ciclo é `deprovisioned` +
  crypto-shredding (ADR-0014).
- **R6** Nenhuma FK atravessa organizações. Verificado por teste automatizado do esquema.

## 4. Isolamento em duas barreiras

**Barreira 1 — aplicação.** Todo acesso passa por repositório com contexto de tenant
obrigatório. Não existe construtor de repositório sem tenant; consulta *cross-tenant*
(relatórios globais) usa um tipo distinto e explícito, com autorização própria e auditoria.

**Barreira 2 — banco (RLS).** Políticas de Row-Level Security por `organization_id`, com
o parâmetro de sessão definido pela aplicação a cada transação. Se a barreira 1 falhar, a
barreira 2 nega. Papel da aplicação **não** tem `BYPASSRLS`.

**Teste de invariante:** suíte que executa consultas com contexto de tenant A tentando
alcançar registros de B — falha do teste quebra o build.

## 5. Convivência XORM ↔ pgx (ADR-0016)

- Tabelas herdadas do upstream: XORM.
- Tabelas novas (membership, auditoria, concessões privilegiadas, políticas): **pgx com SQL
  explícito**.
- **Regra de transação:** uma operação de negócio que toque ambos os mundos abre **uma única
  transação** com a conexão compartilhada; é proibido abrir transações independentes em
  camadas diferentes para a mesma operação.
- Operações que precisam de garantia com sistemas externos (PDP, cofre) usam **outbox
  transacional**, nunca chamada remota dentro da transação de banco.

## 6. Migração de dados do upstream

1. Inventariar identidades duplicadas por `email_hash` entre organizações.
2. **Fusão** de duplicatas em identidade única, gerando memberships correspondentes — com
   relatório de conflito para revisão manual (fusão automática silenciosa é proibida).
3. Migrar credenciais para a identidade fundida, preservando fatores MFA registrados.
4. Backfill de `organization_id` e ativação de RLS por tabela, em ordem controlada.
5. Cifrar campos pessoais e gerar chaves por titular (ADR-0014).
6. Validação: contagem por tenant, teste de travessia, verificação de que nenhuma identidade
   perdeu fator MFA.

Migração é **irreversível na prática** após a fusão: exige backup verificado e ensaio em cópia
de produção antes da execução.

## 7. Considerações de desempenho

- Índices por `(organization_id, ...)` em todas as consultas quentes.
- `email_hash` indexado para login sem descriptografar.
- Cache de memberships por identidade com invalidação em mudança de status.
- Particionamento reservado à auditoria (RFC-0003); demais tabelas não particionadas na v1.
