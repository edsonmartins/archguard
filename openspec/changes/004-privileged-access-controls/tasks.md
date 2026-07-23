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
- [x] **T-008** Exigir justificativa vinculada a incidente na solicitação. *(`PrivilegedGrant`
      ganha `Justification` + `IncidentRef` (vazios em grant normal; LGPD: justificativa é conteúdo
      do tenant, não indexada por pessoa; incidente é referência opaca de chamado). `NewBreakglassRequest`
      exige AMBOS não-vazios (`ErrInvalidGrant`) — acesso emergencial sem motivo declarado e sem
      incidente a que se vincular é recusado. Nasce `requested` origem breakglass. Teste: recusa sem
      justificativa/incidente; aceita com ambos. Gate verde.)*
- [x] **T-009** Integrar step-up WebAuthn obrigatório (pacote 005) e recusar TOTP. *(`PassStepUp`
      passa a consumir o resultado do step-up do pacote 005 — `PassStepUp(provenAAL, phishingResistant)`
      (o chamador passa `session.ProvenAAL`/`session.PhishingResistant()` após o `AuthSession.StepUp`).
      RECUSA fator não resistente a phishing (`ErrStepUpNotPhishingResistant`): TOTP (AAL2, não
      phishing-resistant) não qualifica para break-glass, só WebAuthn (cenário "Fator insuficiente").
      Exige também AAL≥2. Step-up recusado não muda o estado. Teste: TOTP recusado, WebAuthn aceito.
      Gate verde.)*
- [x] **T-010** Implementar aprovação de N pares com validação de aprovadores distintos. *(Distinção
      já em `Approve` (T-007). Acrescenta: (a) autoaprovação recusada — `Approve` rejeita
      approver==SubjectMembershipID (o solicitante) com `ErrSelfApproval` (cenário "Autoaprovação");
      (b) `BreakglassPolicy{RequiredApprovals}` + `NewBreakglassPolicy(required, production)` que
      recusa ZERO aprovadores em produção (`ErrZeroApproversInProduction`, cenário "Zero aprovadores
      em produção") — o domínio recebe `production bool` (o wiring mapeia `deploy.Profile`),
      mantendo-se puro; negativo sempre inválido; zero permitido só fora de produção. `DefaultBreakglassApprovals=2`.
      Testes: autoaprovação recusada e não registrada; zero em produção rejeitado, fora permitido.
      Gate verde.)*
- [x] **T-011** Implementar alerta em tempo real na solicitação (SMTP/webhook). *(`PrivilegedGrant.RequestedNotification()`
      constrói o alerta (kind `breakglass.requested`, incidente + alvo, SEM a justificativa — evita
      PII no canal). `BreakglassRequester` (serviço de domínio sobre o porto `Notifier`): `Request(...)`
      emite o alerta IMEDIATAMENTE, no momento da solicitação, ANTES de qualquer aprovação (a
      solicitação nasce em requested, zero aprovações — cenário "Alerta na solicitação"); se o alerta
      não pôde ser entregue, a solicitação FALHA (não prossegue silenciosa). O porto real SMTP/webhook
      é a implementação do `Notifier` (dev = fake; produção = pacote 010). Também: gate fail-closed do
      canal (`Available` → `ErrNoNotificationChannel`, cenário "Canal indisponível") — inseparável de
      emitir o alerta; a perna de auditoria fail-closed fecha no T-013. Testes: alerta na solicitação
      sem PII, negado sem canal, falha se alerta não entregue. Gate verde.)*
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
