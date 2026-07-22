# Tasks — 005 · MFA obrigatório e step-up

- [x] **T-001** Modelar fatores por identidade com metadados de tipo e AAL. *(Refinação do
      modelo de fator do 002 T-005 — `internal/domain/credential.go` já tinha FactorType/AAL/forma
      INV-7/Params. Adicionado o que o step-up (ADR-0010) exige. Decisão do arquiteto: AAL por
      credencial com TETO por tipo. `MaxAAL(FactorType)`: WebAuthn≤AAL3, TOTP/recovery≤AAL2,
      senha≤AAL1. `Credential.PhishingResistant()` = só WebAuthn (gate L3). `Credential.Strong()`
      = WebAuthn OU TOTP (para "MFA obrigatório"; senha/recovery não contam). `SetAssurance(aal)`
      recusa nível acima do teto (`ErrAssuranceExceedsCeiling`) — a registração (T-002) promove
      WebAuthn a AAL3 com evidência de user-verification/atestação. `WellFormed` passa a rejeitar
      AAL acima do teto: um TOTP forjado com AAL3 NÃO é WellFormed — a base estrutural de "TOTP
      recusado em L3". Sem esquema novo (cabe em credential.aal/Params). Testes: teto por tipo,
      phishing-resistant/strong por tipo, SetAssurance e WellFormed rejeitam AAL acima do teto.
      Gate verde.)*
- [x] **T-002** Implementar registro e autenticação WebAuthn (múltiplos autenticadores).
      *(Dependência: `go-webauthn/webauthn` v0.10.2 já estava na árvore (Casdoor a usa) — NÃO é
      dependência nova; license-gate inalterado. Adapter `internal/adapters/webauthn`: `Service`
      (RP configurável: RPID/RPDisplayName/origins), `User` (handle = subject OPACO, credenciais
      de múltiplos autenticadores; exclude-list impede re-registro). `BeginRegistration`/
      `FinishRegistration` → `domain.Credential` WebAuthn (só material público, INV-7) com AAL
      atribuído: user-verified E NÃO-backup-eligible (hardware) = AAL3; passkey sincronizada ou
      sem UV = AAL2 (distinção do ADR-0010). `BeginLogin`/`FinishLogin` verificam a asserção e
      devolvem a credencial usada + sign_count novo + `CloneWarning` (contador retrocedeu = possível
      clone, o chamador decide o risco, nunca ignora). Recebe `io.Reader` (parse+CreateCredential/
      ValidateLogin) — testável sem http.Request. **Testado com autenticador virtual ES256
      próprio** (crypto/ecdsa + fxamacker/cbor, ambos já deps; sem dep de teste nova): ciclo
      COMPLETO registro→login real, hardware→AAL3 (forma INV-7), passkey sincronizada→AAL2,
      challenge errado recusado. Gate verde.)*
- [x] **T-003** Implementar TOTP como fallback com restrição de nível. *(Dependência:
      `pquerna/otp` v1.4.0 já estava na árvore (Casdoor usa em `object/mfa_totp.go`) — NÃO é
      nova. Adapter `internal/adapters/totp`: `Service(issuer, vault domain.SecretStore)`.
      Perfil SHA-1/6-dígitos/30s (interopera com todo app autenticador), skew=1 (uma janela de
      drift, limita replay). Cerimônia em DOIS passos: `BeginEnrollment` gera a semente e a
      mantém EFÊMERA em memória (nunca persistida/logada; só o provisioning URI é mostrado uma
      vez ao dono via TLS); `FinishEnrollment` só custodia no cofre APÓS o usuário provar posse
      com um código válido — semente não confirmada é descartada, cofre é chamado FORA de tx
      (RFC-0004 §4), e se a construção da credencial falhar compensa com Delete (sem semente
      órfã). Credencial resultante = TOTP AAL2, forma INV-7 (só SecretRef). `Verify` resolve a
      semente do cofre só durante a checagem; falha do cofre é ERRO (fail-closed, INV-6),
      distinta de código errado (negação ok=false). **Restrição de nível**: TOTP é AAL2 por
      construção (teto do T-001), não é phishing-resistant e `SetAssurance(AAL3)` é recusado —
      logo nunca satisfaz L3 (cenário "TOTP em operação L3"). SMS é estruturalmente impossível
      (não há FactorType nem construtor — cenário "SMS como fator → rejeitado"). Testes: ciclo
      registro→verificação, código errado não custodia, TOTP não sobe a AAL3, falha de cofre é
      erro, SMS não é fator. Gate verde.)*
- [x] **T-004** Implementar códigos de recuperação de uso único com invalidação em massa.
      *(`internal/domain/recovery.go` — cripto pura de stdlib, sem dep nova, no domínio como
      `audit_chain.go`. `GenerateRecoveryCodes(id, n)` gera N códigos de 80 bits (base32
      minúsculo agrupado, alfabeto sem 0/1/8/9), devolve o texto plano (mostrado UMA vez, nunca
      persistido/logado) + credenciais que carregam só o verifier SHA-256 (INV-7, AAL2). Entropia
      alta ⇒ SHA-256 sem KDF lento é seguro. `MatchRecoveryCode(creds, input)` normaliza a entrada
      (maiúsc/hífen/espaço), compara em TEMPO CONSTANTE (subtle) sem early-exit (timing não
      revela posição), devolve o id da credencial casada; no-match é `ErrNoRecoveryCode` (negação,
      não erro). **Uso único**: o chamador consome exatamente a credencial casada (remove-a);
      reapresentar o mesmo código não casa mais. **Invalidação em massa**: emitir um conjunto novo
      substitui TODAS as credenciais de recuperação da identidade numa transação — todo código
      antigo para de funcionar de uma vez. Testes: forma INV-7, casamento robusto a formatação,
      no-match é negação, uso único, invalidação em massa, faixa de quantidade. Gate verde.)*
- [x] **T-005** Implementar cálculo de `acr`/`amr`/`auth_time` na sessão. *(A sessão passa a
      carregar o contexto de autenticação: `AuthTime` (OIDC `auth_time`, distinto de `created_at`
      — sobrevive a reautenticações do step-up) e `AuthMethods` (os TIPOS de fator provados, na
      ordem — a FONTE de verdade). `SetAuthContext(at, methods)` é o portão de HONESTIDADE do
      acr: recusa registrar métodos que não sustentam o `ProvenAAL` (ex.: AAL2 só com senha,
      teto aal1) — o acr afirmado nunca excede os fatores usados. Derivados puros no domínio:
      `ACR()` = o token `aal*`; `AMR()` = tokens RFC 8176 (pwd/otp/hwk, dedup na ordem provada)
      + `mfa` quando ≥2 tipos distintos; recovery não tem token amr (fallback break-glass não é
      método anunciado). Persistência (migração 0022): `auth_time timestamptz` (backfill =
      created_at; DEFAULT now() no login via COALESCE) e `auth_methods text[]` com CHECK que só
      admite tipos de fator conhecidos — barra "sms" no banco (cenário "SMS como fator →
      rejeitado"). Store: Create/Get/ListActive/scan threadam os dois campos. Testes de domínio
      (acr/amr por combinação, honestidade, fail-closed sem contexto) e de integração PG
      (round-trip auth_time+métodos, CHECK recusa 'sms'). Gate verde. Consumo nos claims OIDC =
      pacote 006.)*
- [x] **T-006** Implementar metadado de classificação de nível por operação da API. *(O
      MECANISMO; a varredura de todas as ops e o invariante que quebra o build são o T-017.
      `internal/domain/assurance.go` REUSA o tipo `AssuranceLevel` (L1/L2/L3) já definido em
      `audit_event.go` — há UMA noção de nível no domínio, partilhada pelo catálogo de ações de
      auditoria e pelo middleware de step-up. Acrescenta a POLÍTICA: `Valid()` (zero-value NÃO é
      válido — nada de default implícito), `RequiredAAL()` L1→AAL1, L2→AAL2, L3→AAL3,
      `RequiresPhishingResistant()` só L3; **fail-closed**: nível não reconhecido exige o mais forte (AAL3 + phishing-resistant),
      nunca o mais fraco. `Satisfies(provenAAL, phishingResistant)` checa AAL + resistência a
      phishing (frescor é composto pelo middleware no T-008). `OperationCatalog` é a fonte única
      de verdade: `Register` recusa id vazio/nível inválido/duplicata; `Level(id)` de operação
      não registrada é `ErrOperationNotClassified` (DENIAL, não miss — o catálogo nunca é o motivo
      de um caminho privilegiado rodar sem proteção); `IDs()` ordenado = o conjunto que o T-017
      compara com o roteador. `AuthSession.PhishingResistant()` deriva dos métodos provados.
      Testes: mapa nível→exigência, fail-closed do nível desconhecido, Satisfies (inclui "TOTP em
      L3" recusado), catálogo register/lookup/duplicata/não-classificada. Gate verde.)*
- [x] **T-007** Implementar middleware de verificação de garantia com erro específico. *(Núcleo
      de decisão no domínio + adaptador HTTP fino. `domain.AssuranceGuard.Authorize(opID, session)`
      é fail-closed em todo eixo: op não classificada → `ErrOperationNotClassified` (denial);
      sessão nil/não-ativa ou abaixo do nível (AAL ou resistência a phishing) →
      `*InsufficientAssuranceError`, o ERRO ESPECÍFICO que carrega a operação, o nível exigido, o
      `RequiredACR` (o acr a alcançar), `NeedsPhishingResistant` e o acr atual — tudo que o cliente
      precisa para o step-up. `internal/http/assurance.go`: `AssuranceMiddleware.Require(opID, next)`
      resolve a sessão (via `SessionResolver`, posto pela camada de auth), chama o guard e, na
      recusa por garantia, responde **401 com desafio RFC 9470** (header
      `WWW-Authenticate: Bearer error="insufficient_user_authentication", acr_values="aal3"` +
      corpo JSON com required_level/acr_values/needs_phishing_resistant) — o handler protegido NÃO
      roda. Op não classificada = defeito de fiação → 500 (fail-closed, o T-017 é o gate que
      impede isso em produção). Testes de domínio (allow, recusa específica, não-classificada, nil/
      revogada) e HTTP (allow roda handler; TOTP em L3 → 401 com acr_values=aal3 e handler não
      roda; sem sessão; não-classificada → 500). Frescor (sessão antiga) é o T-008, que compõe
      sobre este guard. Gate verde.)*
- [x] **T-008** Implementar avaliação de frescor no momento da operação. *(Frescor como eixo
      separado que o guard compõe. `AssuranceLevel.Fresh(authTime, now)`: L1 sempre fresco (sessão
      válida basta); L2 janela de 12h; L3 janela CURTA de 5min (reautenticação recente) — defaults
      da plataforma, que a política por organização (T-010) só pode APERTAR. Fail-closed: auth_time
      zero ou no futuro nunca é fresco; nível desconhecido usa a janela curta. `Authorize` ganha o
      parâmetro `now` (injetado pelo chamador — domínio segue sem relógio): após checar AAL/phishing,
      recusa a sessão obsoleta com `InsufficientAssuranceError{Stale: true}` — o cenário "Operação
      L3 com sessão antiga": a sessão TEM o fator certo mas autenticou há muito, então exige step-up
      mesmo assim (RequiredACR continua aal3). O step-up (SetAuthContext renovando auth_time) restaura
      o frescor e a operação passa. Middleware injeta `now` (time.Now, sobreponível em teste) e emite
      o mesmo desafio RFC 9470. Testes de domínio (Fresh por nível + fail-closed, recusa Stale,
      step-up restaura) e HTTP (sessão antiga → 401 com acr_values). Gate verde.)*
- [x] **T-009** Implementar fluxo de step-up e retomada da operação original. *(A transição de
      sessão que torna a retomada possível. `AuthSession.StepUp(at, aal, methods)`: eleva (ou
      mantém) o AAL comprovado e renova o contexto de autenticação (auth_time, métodos), restaurando
      o frescor. Recusa em sessão revogada; recusa REDUZIR a garantia (`ErrStepUpLowersAssurance` —
      um L3 obsoleto reautentica e SEGUE L3, só renova o frescor); não infla acr além dos fatores
      (checagem de honestidade do SetAuthContext, com rollback do nível se os métodos não sustentam).
      Após o step-up, `ACR()` reflete o nível obtido e `Fresh()` é verdadeiro. **Retomada sem perda
      de contexto**: a MESMA operação (mesmo id, tenant e parâmetros) que fora recusada agora passa
      no guard — só a garantia da sessão subiu. Persistência: `IdentitySessionStore.SaveStepUp`
      atualiza proven_aal/auth_time/auth_methods só na sessão ATIVA (tenant e token_generation
      intactos). Testes de domínio (fluxo recusa→step-up→retomada com acr/amr renovados, refresh de
      frescor, guardas de redução/inflação/revogada) e integração PG (step-up TOTP→WebAuthn persiste
      e a releitura reflete aal3/auth_time/métodos). A cerimônia WebAuthn/HTTP em si reusa o
      adaptador do T-002 + o middleware do T-007. Gate verde.)*
- [x] **T-010** Implementar política de MFA por organização. *(Substitui o adapter provisório
      `tenantswitch.ProfilePolicy` pela política real, sobre o port `TenantAuthPolicy` já existente.
      `domain.OrgMFAPolicy{OrganizationID, MinimumAAL}`: o PISO de garantia do tenant — AAL2 = "MFA
      obrigatório", AAL3 = "WebAuthn obrigatório" (`RequiresPhishingResistant`). `SatisfiedBy`
      fail-closed. `DefaultOrgMinimumAAL = AAL1` é o baseline da plataforma quando a org não
      declarou política (ausência de linha = decisão de baseline, NÃO fail-open — os níveis por
      operação continuam valendo por cima; o piso só SOBE). Migração 0023: `organization_mfa_policy`
      (PK org, minimum_aal com CHECK, RLS FORCE por `app.current_organization`). Store tenant-scoped
      `OrgMFAPolicyStore` (Get devolve default se não há linha, ERRO se a query falha — nunca trata
      política ilegível como default; Set faz upsert; ambos com guarda Barreira 1 `ErrCrossTenantPolicy`).
      `OrgPolicyAuthority` implementa `TenantAuthPolicy.RequiredAAL` abrindo leitura no contexto de
      tenant da org consultada (RLS admite exatamente aquela linha), default AAL1, fail-closed em
      falha de store. `ALTER DEFAULT PRIVILEGES` do roles.sql já cobre a tabela nova (não é
      auditoria). Testes: domínio (construção/default/SatisfiedBy) e integração PG (default→declara
      AAL3→autoridade reflete; outra org no baseline; upsert; guarda cross-tenant). Precedência
      "mais restritiva vence" na troca é o T-011. Gate verde.)*
- [ ] **T-011** Implementar precedência "mais restritiva vence" na troca de tenant.
- [ ] **T-012** Implementar estado `enrollment_required` bloqueante.
- [ ] **T-013** Implementar processo de recuperação com aprovação de pares.
- [ ] **T-014** Implementar limitação de taxa e bloqueio progressivo.
- [ ] **T-015** Implementar detecção de credential stuffing com alerta.
- [ ] **T-016** Auditar todos os eventos de MFA (incluindo remoção de fator).
- [ ] **T-017** Classificar todas as operações existentes; falhar o build se houver
      operação sem classificação.
- [ ] **T-018** Teste: operação L3 com sessão antiga exige reautenticação.
- [ ] **T-019** Teste: TOTP recusado em operação L3.
- [ ] **T-020** Teste: nenhum caminho de reset administrativo silencioso de fator.

## Gate de verificação
100% das operações classificadas; nenhuma operação L3 acessível sem WebAuthn recente; teste de
ausência de backdoor de recuperação verde.
