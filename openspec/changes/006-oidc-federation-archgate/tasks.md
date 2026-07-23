# Tasks — 006 · Federação OIDC

- [x] **T-001** Especificar e versionar o contrato de claims v1 (OpenAPI + documentação). *(Contrato
      versionado como TIPO de domínio verificável + doc. `internal/domain/oidc_claims.go`:
      `OIDCClaims` (RFC-0006 §3, agnóstico de fornecedor — nada do fork vaza), tags JSON exatas do
      contrato, `OIDCClaimsVersion="v1"` no claim `archguard_claims_version` (mudança de semântica de
      claim v1 exige NOVA versão, nunca redefinição silenciosa). `WellFormed()` é o gate estrutural
      antes de assinar: obrigatórios presentes (iss/sub/aud/org/mid/acr/amr/auth_time/sid/versão),
      acr é nível válido, janela iat/exp coerente. E-mail/act/pcid/grant_ref/groups/roles opcionais
      (omitempty) — e-mail NUNCA aparece sem escopo (I-3.2). Reusa `ActClaim` do pacote 004. Doc:
      `docs/oidc/CLAIMS-v1.md` (tabela de claims + regras invariantes + ciclo de vida; RFC-0006
      governa). Testes: WellFormed aceita/rejeita por claim, contrato JSON usa os nomes certos e não
      vaza opcionais/e-mail. Gate verde.)*
- [x] **T-002** Implementar emissão dos claims `org`, `mid`, `acr`, `amr`, `sid`. *(`BuildOIDCClaims(OIDCClaimsInput)`
      monta o claim set v1 a partir da sessão autenticada: `org`/`mid` do TENANT ATIVO
      (`Session.ActiveTenant()`), `acr` de `Session.ACR()` (L1/L2/L3 após a reconciliação), `amr` de
      `Session.AMR()` (RFC 8176), `auth_time`/`sid` da sessão, `sub` opaco do input. Recusa sessão
      sem tenant ativo (pending/revogada não emite token — mesmo gate da emissão) e TTL de access
      fora de [5,15] min (RFC-0006 §5). Valida com `WellFormed` antes de retornar — claim set
      malformado nunca sai. `act`/`pcid`/`grant_ref`/`email` ficam para T-003/004/006. Testes:
      emissão padrão (org/mid/acr/amr/auth_time/sid do tenant ativo), recusa de sessão pendente e de
      TTL longo. Gate verde.)*
- [x] **T-003** Implementar `pcid` (correlação de sessão privilegiada) e sua propagação. *(`NewPCID()`
      gera um id opaco de 128 bits (prefixo `pcid_`, base32), não-pessoal, estável pela vida da sessão
      privilegiada. Campo `PCID` no `OIDCClaimsInput`; o builder carrega no claim `pcid`. Propagação:
      o MESMO valor é gravado em `AuditContext.PrivilegedCorrelationID` (já existente, pacote 003) —
      é isto que une a trilha do ArchGuard à do componente numa linha do tempo (cenário "Linha do
      tempo unificada"). Vazio em sessão comum. Testes: pcid único/opaco, token o carrega, mesmo
      valor no contexto de auditoria. Gate verde.)*
- [x] **T-004** Implementar `act` para delegação e `grant_ref` para concessões. *(Campos `Act *ActClaim`
      e `GrantRef` no `OIDCClaimsInput`; o builder os carrega nos claims `act`/`grant_ref`. `act` vem
      de `Delegation.TokenClaims().Act` (pacote 004, ator real RFC 8693); `grant_ref` = id da
      `PrivilegedGrant`. Validação: `act` presente DEVE ter `sub` (delegação quebrada — act sem sub —
      nunca é montada). Testes: token de delegação carrega act (nomeia o ator real) + grant_ref;
      act sem sub recusado. Gate verde.)*
- [x] **T-005** Tornar PKCE obrigatório; remover fluxos implicit e ROPC. *(`internal/domain/oidc_flow.go`,
      política de fluxo pura. `OAuthFlow` só tem authorization_code e device_code — implicit e ROPC
      NÃO são valores suportados (recusados, não desabilitados por config). `ValidateResponseType`
      recusa qualquer coisa != "code" (barra implicit/hybrid com token); `ValidateGrantType` recusa
      `password` (ROPC) e grants não suportados; `ValidatePKCE` exige `code_challenge` não-vazio +
      método `S256` (plain recusado). `ValidateAuthorizationCodeRequest` combina response_type=code
      + PKCE S256, fail-closed antes de emitir código. Cobre "PKCE ausente" e "Fluxo obsoleto".
      Testes: PKCE ausente/plain recusado, implicit/ROPC recusados, suportados passam. Gate verde. A
      IMPOSIÇÃO no endpoint de autorização herdado (controllers Casdoor) é wiring do T-013+.)*
- [x] **T-006** Implementar audiência específica por cliente e escopo mínimo. *(Cada token já tem
      um único `aud` (T-002). `ValidateAudience(tokenAud, componentAud)` é a checagem do lado do
      componente: aceita só se `aud` == a própria audiência — token de A em B é recusado
      (`ErrAudienceMismatch`, cenário "Reuso entre componentes"); fail-closed com aud vazia.
      `MinimalScope(requested, allowed)` = interseção determinística (menor privilégio — nunca um
      escopo não pedido nem não permitido). E-mail no builder só sob `ScopeEmail` em `GrantedScopes`
      (cenário "Dado pessoal restrito" / I-3.2): sem o escopo, o claim `email` não é emitido mesmo
      com e-mail no input. Testes: audiência A≠B recusada, escopo mínimo, e-mail só sob escopo. Gate
      verde.)*
- [x] **T-007** Implementar rotação de refresh token com detecção de reuso. *(`internal/domain/refresh_token.go`,
      domínio puro. `NewRefreshSecret()` entrega o segredo UMA vez (prefixo rt_, 160 bits) e o hash
      SHA-256 guardado (INV-7 — segredo nunca persistido; casa por hash). `RefreshToken` pertence a
      uma FAMÍLIA (`FamilyID` = cadeia de rotações da sessão); status active→rotated|revoked. `Rotate`
      (só de token ativo, `ErrRefreshNotActive`) marca o atual rotated e devolve o SUCESSOR ativo na
      mesma família (cenário "Renovação normal"). `CheckReuse` = o sinal de reuso: apresentar um token
      rotated/revoked é `ErrRefreshReuse` — o chamador revoga a família inteira (T-008). `Usable`/`Expired`
      cobrem expiração. Testes: segredo/hash único, rotação (anterior invalidado, sucessor na família),
      reuso detectado (rotated e revoked), expiração. Gate verde. A revogação de família + evento de
      severidade alta é o T-008.)*
- [ ] **T-008** Implementar revogação em cascata da família de tokens.
- [ ] **T-009** Implementar back-channel logout OIDC.
- [ ] **T-010** Implementar introspecção com TTL curto para componentes sem logout.
- [ ] **T-011** Implementar rotação de JWKS com sobreposição e `kid`.
- [ ] **T-012** Bloquear operações L3 originadas de device flow.
- [ ] **T-013** Registrar clientes: Warpgate, Guacamole, NetBird, OpenBao, proxy Oracle JDBC.
- [ ] **T-014** Mapear claims → políticas do OpenBao a partir da mesma fonte de papéis.
- [ ] **T-015** Adaptação de borda para limitações do Guacamole (documentada).
- [ ] **T-016** Implementar suíte de conformidade por componente.
- [ ] **T-017** Integrar a suíte como gate de release no CI.
- [ ] **T-018** Teste: token de um componente recusado por outro (audiência).
- [ ] **T-019** Teste: logout no ArchGuard encerra sessões nos componentes.
- [ ] **T-020** Teste: correlação `pcid` reconstrói a linha do tempo ponta a ponta.

## Gate de verificação
Suíte de conformidade verde para todos os componentes; reuso de refresh detectado e punido;
linha do tempo correlacionada demonstrada em ambiente de homologação.
