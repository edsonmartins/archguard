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
- [x] **T-011** Implementar precedência "mais restritiva vence" na troca de tenant. *(Dois eixos.
      (1) NA TROCA: o `TenantSwitcher` (002) já lê a política do destino via `TenantAuthPolicy.
      RequiredAAL` e `SwitchTenant` nega com `ErrStepUpRequired` se o comprovado < exigido — agora
      ligado à política REAL (T-010, `OrgPolicyAuthority`): o cenário "TOTP troca para tenant que
      exige WebAuthn → step-up antes de concluir" funciona por construção (AAL2 < AAL3). (2) NAS
      OPERAÇÕES: `AssuranceGuard.Authorize` ganha o parâmetro `tenantFloor` e COMPÕE o nível da
      operação com o piso do tenant ativo tomando o MAIS restritivo em cada eixo — uma operação L1
      num tenant com piso AAL3 passa a exigir AAL3 (+ phishing-resistant, pois AAL3=WebAuthn), e uma
      operação estrita NÃO é afrouxada por um tenant lasso. Fail-closed: piso indefinido é tratado
      como AAL3. O middleware resolve o piso do tenant ativo (via `TenantFloor`/OrgPolicyAuthority);
      falha ao resolver a política é fail-closed (500). O desafio RFC 9470 informa o acr EFETIVO
      (aal3 pelo piso). Testes de domínio (piso eleva L1, estrita não afrouxa, piso inválido
      fail-closed) e HTTP (piso AAL3 desafia L1; política indisponível → 500). Gate verde.)*
- [x] **T-012** Implementar estado `enrollment_required` bloqueante. *(`domain.RequiresEnrollment(privileged, creds)`:
      um privilegiado sem fator forte (nenhum `Credential.Strong()`) exige enrolamento; não-privilegiado
      nunca. O `privileged` é computado pelo chamador a partir dos papéis no tenant ativo (authz é
      pacote 004/007). `AuthSession.EnrollmentRequired` + Mark/Clear; a sessão entra no estado no
      login. `Operation.AllowedDuringEnrollment` marca as poucas operações de registro de fator que
      seguem permitidas. `AssuranceGuard.Authorize` usa `Lookup` e, ANTES da checagem de garantia,
      se a sessão está em enrolamento e a operação não é exceção → `ErrEnrollmentRequired` (o gate
      precede o de AAL — nem uma leitura L1 é alcançável até haver fator forte). Persistência
      (migração 0024): coluna `enrollment_required` (default false; login LEVANTA); store threada em
      Create/Get/scan + `ClearEnrollment` (limpa na sessão ativa após enrolar). Testes de domínio
      (RequiresEnrollment por combinação; bloqueio de op comum vs permissão de enrolamento; gate
      precede garantia) e integração PG (persiste no login, limpa por ClearEnrollment). Gate verde.)*
- [x] **T-013** Implementar processo de recuperação com aprovação de pares. *(Máquina de estados
      de domínio `RecoveryRequest` (pending→approved→consumed | pending→rejected): justificativa
      OBRIGATÓRIA; limiar de aprovações de PARES distintos (default 2). Separação de deveres: o
      aprovador não pode ser o alvo (`ErrApproverIsTarget`) nem o solicitante (`ErrApproverIsRequester`),
      e não aprova duas vezes (`ErrDuplicateApproval`). `MarkConsumed` (o reset realizado) só é
      válido a partir de approved — nenhum caminho reseta um fator sem passar pela aprovação
      (cenário "reset silencioso"). Persistência (migração 0025): `recovery_request` +
      `recovery_approval` (PK composta = aprovadores distintos no banco), RLS FORCE por org. Store
      tenant-scoped `RecoveryRequestStore` (Create/Get com aprovações/SaveDecision upsert; guarda
      Barreira 1). Testes de domínio (limiar, separação de deveres, consumo exige aprovação,
      rejeição terminal) e integração PG (ciclo completo: alvo abre, dois pares distintos aprovam em
      transações separadas recarregando o estado, aprovada→consumida). Auditoria/notificação do
      processo = T-016. Gate verde.)*
- [x] **T-014** Implementar limitação de taxa e bloqueio progressivo. *(`domain.Throttle{Failures,
      LockedUntil}` por identidade: `Locked(now)` é o gate antes de validar credencial; `RecordFailure`
      incrementa e, a partir do limiar (5 falhas), aplica bloqueio que DOBRA a cada falha adicional
      (base 30s, teto 1h) — brute force fica impraticável e alguns erros honestos não custam nada;
      `RecordSuccess` zera o estado. `ThrottleStore` port fail-closed (falha de store nega, nunca
      bypassa). Persistência (migração 0026): `auth_throttle` (PK identity, RLS FORCE pelo eixo
      `app.current_identity`). Store identity-scoped (Get devolve zero sem linha, upsert em Save,
      guarda Barreira 1). Testes de domínio (progressão, teto, sucesso zera) e integração PG
      (persiste falhas→bloqueio→sucesso zera; guarda cross-identity). Evento de auditoria do
      bloqueio = T-016. Gate verde.)*
- [x] **T-015** Implementar detecção de credential stuffing com alerta. *(`domain.StuffingDetector`:
      rastreia, por ORIGEM, as identidades DISTINTAS que uma falha de login atingiu numa janela
      deslizante (5min); quando uma origem cruza o limiar (10 identidades distintas) levanta o
      alerta — o padrão distribuído, diferente do brute force de UMA conta (que é o throttle T-014).
      A origem é uma chave OPACA fornecida pelo chamador (hash do endereço, nunca IP cru aqui —
      contexto de acesso é do evento de auditoria, RFC-0003; INV-7). `Observe` poda entradas fora da
      janela, conta distintas, dispara UMA vez por origem por janela (sem spam), e volta a poder
      alertar quando a janela drena. Seguro para uso concorrente (mutex); detector in-process = seam
      da versão durável/cross-replica (observabilidade, pacote 010). Testes (alerta no limiar e uma
      só vez; só distintas contam; origens independentes; poda da janela) passam com `-race`. Gate
      verde.)*
- [x] **T-016** Auditar todos os eventos de MFA (incluindo remoção de fator). *(Vocabulário de
      auditoria: 8 ações novas no catálogo FECHADO (`audit_event.go`) — `factor.enroll` (L2),
      `factor.remove` (L3), `auth.stepup` (L1), `auth.lockout` (L1), `auth.stuffing_alert` (L1),
      `recovery.request` (L2), `recovery.approve` (L3), `recovery.reset` (L3); o teste do catálogo
      (aberto) valida que cada uma tem nível. Cenário "Remoção de fator" WIRED de ponta a ponta:
      `CredentialStore.Remove` (predicado identity_id impede remover credencial de outrem) +
      `FactorRemover.RemoveStrongFactor` grava `factor.remove` com ator (principal do contexto),
      alvo (subject OPACO da identidade afetada) e resultado, ATOMICAMENTE na transação da remoção
      (fail-closed via emitAudit: sem principal ⇒ ErrNoPrincipal ⇒ rollback, remoção não auditável
      não acontece, I-5.4). Testes de integração PG (remoção audita com ator/alvo; sem principal
      desfaz). A emissão dos demais eventos (enroll/step-up/lockout/stuffing/recovery) usa estas
      ações onde cada fluxo é montado (login/enrolamento = pacote 006/008; notificação da identidade
      afetada = porto de notificação, pacote 010). Gate verde.)*
- [x] **T-017** Classificar todas as operações existentes; falhar o build se houver
      operação sem classificação. *(`internal/domain/operation_catalog.go` é a FONTE ÚNICA de
      classificação: `classifiedOperations` lista EXPLICITAMENTE cada operação gated com seu nível
      (ids reusam os verbos de auditoria onde há trilha, para endpoint e verbo nunca divergirem;
      alguns reads têm id próprio). `BuildOperationCatalog()` falha se qualquer operação for
      malformada/duplicada. `operationExemptActions` isenta, COM MOTIVO, os verbos que não são
      operações gated (login pré-auth, login.denied de resultado, a própria cerimônia de step-up,
      lockout/stuffing emitidos pelo sistema). Invariante INV-8 (`test/invariants/inv8_*`, entra no
      `make invariants` do gate): (1) o catálogo constrói e toda operação tem nível válido; (2)
      COMPLETUDE — todo verbo de auditoria é classificado como operação OU isento, nunca ambos nunca
      nenhum: adicionar um verbo novo sem classificar/isentar QUEBRA O BUILD (cenário "Operação sem
      classificação"); (3) consistência — operação cujo id é verbo tem o mesmo nível do verbo. Gate
      verde.)*
- [x] **T-018** Teste: operação L3 com sessão antiga exige reautenticação. *(Teste de aceitação
      `TestAcceptanceL3StaleSessionRequiresReauth` contra o catálogo canônico (BuildOperationCatalog,
      a mesma classificação de produção): sessão WebAuthn AAL3 mas ANTIGA em `privileged.session.open`
      (L3) → recusa `Stale` exigindo acr aal3; após StepUp a operação passa. Também coberto por
      `TestGuardDeniesStaleSession`/`TestAssuranceMiddlewareChallengesStale`.)*
- [x] **T-019** Teste: TOTP recusado em operação L3. *(`TestAcceptanceTOTPDeniedAtL3` contra o
      catálogo canônico: sessão TOTP AAL2 em `audit.export` (L3) → recusa exigindo explicitamente
      fator resistente a phishing (aal3). Também: `TestTOTPCannotSatisfyL3` (estrutural, T-001) e
      `TestGuardDeniesWithSpecificError`.)*
- [x] **T-020** Teste: nenhum caminho de reset administrativo silencioso de fator.
      *(`TestAcceptanceNoSilentAdminFactorReset`: os dois caminhos são estruturalmente
      não-silenciosos — (a) `factor.remove` é operação L3 e NÃO permitida em enrolamento (auditada
      + step-up; atomicidade da auditoria provada em `TestFactorRemoverAuditsRemoval` e o fail-closed
      sem principal em `TestFactorRemoverFailsClosedWithoutPrincipal`); (b) recuperação: `MarkConsumed`
      exige aprovado — sem aprovação de pares não há reset. Também `TestRecoveryConsumeRequiresApproval`.)*

## Gate de verificação
100% das operações classificadas; nenhuma operação L3 acessível sem WebAuthn recente; teste de
ausência de backdoor de recuperação verde.

**FECHADO (2026-07-22).** 20/20 tarefas [x]. Gate completo verde (`make lint/invariants/deps-check/
sbom/build` + suíte `go test ./internal/... ./cmd/... ./test/...` contra PG 15 real).
- **100% das operações classificadas**: `BuildOperationCatalog` é a fonte única; o invariante INV-8
  (`test/invariants/inv8_*`, no `make invariants`) rejeita o build se um verbo de auditoria não for
  classificado nem isento com motivo (T-017).
- **Nenhuma operação L3 sem WebAuthn recente**: teto de AAL por tipo (TOTP≤AAL2, T-001) + `Satisfies`
  exige phishing-resistant em L3 + frescor L3 de 5min (T-008); aceitação em T-018/T-019.
- **Sem backdoor de recuperação**: remoção de fator é L3 auditada fail-closed (T-016) e a recuperação
  exige aprovação de pares distintos (`MarkConsumed` só após aprovado, T-013); aceitação em T-020.
- Dependências: NENHUMA nova (go-webauthn e pquerna/otp já eram do Casdoor).
- Impls in-process marcadas como seam do durável (StuffingDetector → observabilidade, pacote 010).
- Emissão de auditoria de login/enrolamento/step-up e notificação da identidade afetada wireadas
  onde os fluxos são montados (pacotes 006/008/010); vocabulário (ações) e o caminho de remoção já
  prontos.
