# Tasks — 005 · MFA obrigatório e step-up

- [ ] **T-001** Modelar fatores por identidade com metadados de tipo e AAL.
- [ ] **T-002** Implementar registro e autenticação WebAuthn (múltiplos autenticadores).
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
