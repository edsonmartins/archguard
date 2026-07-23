# Tasks — 004 · Controles de acesso privilegiado

- [x] **T-001** Modelar `privileged_grant` (sujeito, alvo, janela, origem, aprovações, status).
      *(`internal/domain/privileged_grant.go`, domínio puro. `PrivilegedGrant`: sujeito =
      `SubjectMembershipID` (R2 — privilégio por MEMBERSHIP, nunca segue a pessoa a outra org),
      `GrantTarget{Type,ID,Scope}` (ativo opaco + escopo), janela `[NotBefore, ExpiresAt)`, `Origin`
      (normal|breakglass), `Status` (requested→awaiting_approval→active→expired|revoked; +denied/
      rejected — a máquina em si é T-007), `RequiredApprovals`/`Approvals`. `NewPrivilegedGrant`
      valida referências, alvo completo, origem e janela POSITIVA; nasce em `requested`. Propriedade
      de segurança central: `Authorizes(now)` avalia autoridade NO MOMENTO DA DECISÃO — status active
      E now estritamente dentro da janela; um grant fora da janela não autoriza NADA mesmo com status
      ainda 'active' (job não materializou) — base do cenário "Token emitido antes da expiração".
      Fail-closed. Timestamps injetados (domínio sem relógio). Testes: construção, validações,
      autorização só dentro da janela. Gate verde.)*
- [x] **T-002** Implementar emissão de token de delegação com claim `act`. *(`internal/domain/
      delegation.go`. `Delegation`: ator real (`RealActorMembershipID`/`RealActorSubject`) + alvo
      impersonado (`TargetIdentityID`/`TargetSubject`), janela, status. NASCE `pending_consent`
      (consentimento é o padrão, ADR-0008 — não há construtor que já inicie ativo; acesso sem
      consentimento só via break-glass). Recusa ator==alvo. `TokenClaims(now)` emite `DelegationTokenClaims`
      só se ATIVA e vigente (`ErrDelegationNotActive`, fail-closed): `sub` = sujeito impersonado,
      `act` = ator real (RFC 8693), `delegated: true` marca o token para o banner (T-005) e o guard
      de escopo (T-003). `AuditActor()` registra AMBOS (aparente = impersonado, `Act` = ator real) —
      não-repúdio reconstrói quem executou. O JWT assinado é o pacote 006; aqui é o conteúdo de
      domínio. Testes: nasce pendente, validações, claims carregam sub+act e recusam fora da janela,
      auditoria registra ambos. Gate verde.)*
- [x] **T-003** Implementar restrições de escopo da delegação (sem admin, sem segredos, sem
      aprovação). *(Cada operação do catálogo canônico ganha `ForbiddenUnderDelegation` — EXPLÍCITO
      e auditável: permitidas sob delegação são só as de suporte/leitura L1 (profile.read,
      session.list, logout, tenant.select, membership.accept); TODO o resto (mutações
      administrativas, segredos/cofre, aprovações de break-glass e de recuperação, ações
      privilegiadas, e até factor.enroll) é proibido. `DelegationScopeGuard.Authorize(opID, delegated)`:
      no-op para sessão comum; para sessão de delegação é fail-closed — não classificada →
      `ErrOperationNotClassified`, proibida → `ErrDelegationScopeExceeded` (a escalada que o
      chamador audita). Lê o MESMO catálogo do guard de garantia, então escopo e classificação nunca
      divergem. Cobre "Tentativa de escalada" e "Tentativa de aprovação". Testes: suporte permitido,
      admin/segredo/aprovação negados, no-op fora de delegação, fail-closed não classificada. INV-8
      segue verde. Gate verde.)*
- [x] **T-004** Implementar fluxo de consentimento do usuário-alvo. *(`Delegation.Consent()` move
      pending_consent→active — o gate que a spec exige antes de iniciar a sessão ("requer
      consentimento do usuário-alvo antes de iniciar a sessão"); `DenyConsent()` move para denied
      (terminal, nunca inicia). Ambas válidas só a partir de pending_consent (`ErrDelegationTransition`).
      Como `TokenClaims` já exige ATIVA, o consentimento é o gate estrutural: sem consentir, nenhum
      token é emitido. O chamador verifica que quem consente É a identidade-alvo. Notificação do
      início é o T-005. Testes: consentimento habilita o token; recusa vai para denied e nunca emite;
      consentir uma delegação já ativa é transição inválida. Gate verde.)*
- [x] **T-005** Implementar notificação ao alvo e banner de sessão delegada. *(Porto `Notifier`
      novo (`internal/domain/notification.go`), DISTINTO do `Alerter` best-effort do 003: sua
      disponibilidade é PRÉ-CONDIÇÃO de fluxos privilegiados — `Available(ctx, org)` é o gate
      fail-closed que o break-glass checa (T-013). `Notification{OrganizationID, Recipient (subject
      opaco), Kind, Detail}` sem dado pessoal. `Delegation.StartedNotification()` notifica o ALVO do
      início nomeando o ator real (cenário "Delegação padrão"); `RevokedNotification()` para a
      revogação; `SessionBanner()` = o banner permanente (ADR-0008: operador nunca esquece que
      impersona) nomeando ambos; o claim `Delegated:true` (T-002) marca o token para o console
      renderizar. Testes: notificação/banner nomeiam ambos, sem PII. A entrega em si é wireada na
      orquestração da delegação (com persistência); porto e conteúdo prontos. Gate verde.)*
- [x] **T-006** Implementar revogação de delegação pelo alvo e pelo administrador. *(`Delegation.Revoke()`
      move uma delegação viva (active ou pending_consent) para revoked, idempotente; disponível a
      AMBOS (a autorização de qual principal pode revogar — alvo ou admin — é do handler; a transição
      é a mesma). Como `TokenClaims` exige ativa, a revogação encerra a sessão delegada IMEDIATAMENTE:
      nenhum token é emitido depois (cenário "Revogação pelo alvo"). Não reativa terminais. Teste:
      revogar interrompe a emissão de token na hora; idempotente. Gate verde.)*
- [x] **T-007** Implementar máquina de estados de break-glass. *(Transições no `PrivilegedGrant`:
      `PassStepUp()` requested→awaiting_approval (o caller já validou fator resistente a phishing,
      T-009; se 0 aprovações — só dev, prod proíbe em T-010 — ativa direto), `Deny()` requested→denied,
      `Approve(approver)` conta aprovadores DISTINTOS (duplicata = `ErrGrantDuplicateApproval`) e ao
      atingir o limiar ativa, `Reject()` awaiting→rejected, `Expire(now)` awaiting/active→expired só
      com janela VENCIDA (sem expiração prematura), `Revoke()` active→revoked (gatilho da cascata,
      T-012). Cada transição valida o estado de origem (`ErrGrantTransition`). Cobre "Solicitação
      completa". Testes: caminho feliz (2 aprovações distintas ativam), duplicata não conta, expirar
      exige janela vencida, revogar exige ativo. (Auto-aprovação/distinção-do-solicitante = T-010;
      justificativa = T-008.) Gate verde.)*
- [ ] **T-008** Exigir justificativa vinculada a incidente na solicitação.
- [ ] **T-009** Integrar step-up WebAuthn obrigatório (pacote 005) e recusar TOTP.
- [ ] **T-010** Implementar aprovação de N pares com validação de aprovadores distintos.
- [ ] **T-011** Implementar alerta em tempo real na solicitação (SMTP/webhook).
- [ ] **T-012** Implementar expiração automática e revogação em cascata das sessões derivadas.
- [ ] **T-013** Implementar fail-closed para ausência de auditoria ou de canal de notificação.
- [ ] **T-014** Implementar registro de revisão pós-uso.
- [ ] **T-015** Implementar tipo de identidade `service` sem login interativo.
- [ ] **T-016** Impedir impersonation de conta de serviço (regra + teste).
- [ ] **T-017** Auditar todos os eventos do ciclo (solicitação, aprovação, uso, expiração,
      revogação, revisão).
- [ ] **T-018** Teste: delegação não escala privilégio nem aprova solicitações.
- [ ] **T-019** Teste: break-glass sem canal de notificação é negado.
- [ ] **T-020** Teste: concessão expirada não autoriza acesso mesmo com token válido em mãos.

## Gate de verificação
Nenhum caminho concede acesso privilegiado sem justificativa, step-up e aprovação; testes de
escalada de privilégio via delegação falham em conceder; auditoria reconstrói ator real em
100% das ações delegadas.
