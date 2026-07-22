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
- [ ] **T-003** Implementar TOTP como fallback com restrição de nível.
- [ ] **T-004** Implementar códigos de recuperação de uso único com invalidação em massa.
- [ ] **T-005** Implementar cálculo de `acr`/`amr`/`auth_time` na sessão.
- [ ] **T-006** Implementar metadado de classificação de nível por operação da API.
- [ ] **T-007** Implementar middleware de verificação de garantia com erro específico.
- [ ] **T-008** Implementar avaliação de frescor no momento da operação.
- [ ] **T-009** Implementar fluxo de step-up e retomada da operação original.
- [ ] **T-010** Implementar política de MFA por organização.
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
