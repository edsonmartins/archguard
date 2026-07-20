# Tasks — 004 · Controles de acesso privilegiado

- [ ] **T-001** Modelar `privileged_grant` (sujeito, alvo, janela, origem, aprovações, status).
- [ ] **T-002** Implementar emissão de token de delegação com claim `act`.
- [ ] **T-003** Implementar restrições de escopo da delegação (sem admin, sem segredos, sem
      aprovação).
- [ ] **T-004** Implementar fluxo de consentimento do usuário-alvo.
- [ ] **T-005** Implementar notificação ao alvo e banner de sessão delegada.
- [ ] **T-006** Implementar revogação de delegação pelo alvo e pelo administrador.
- [ ] **T-007** Implementar máquina de estados de break-glass.
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
