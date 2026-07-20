# Tasks — 006 · Federação OIDC

- [ ] **T-001** Especificar e versionar o contrato de claims v1 (OpenAPI + documentação).
- [ ] **T-002** Implementar emissão dos claims `org`, `mid`, `acr`, `amr`, `sid`.
- [ ] **T-003** Implementar `pcid` (correlação de sessão privilegiada) e sua propagação.
- [ ] **T-004** Implementar `act` para delegação e `grant_ref` para concessões.
- [ ] **T-005** Tornar PKCE obrigatório; remover fluxos implicit e ROPC.
- [ ] **T-006** Implementar audiência específica por cliente e escopo mínimo.
- [ ] **T-007** Implementar rotação de refresh token com detecção de reuso.
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
