# Tasks — 008 · ArchGuard Console

- [ ] **T-001** Bootstrap do projeto (Vite + React 19 + TS strict + Mantine v9 + Archbase).
- [ ] **T-002** Pipeline de geração do cliente a partir do OpenAPI + verificação de defasagem.
- [ ] **T-003** Lint que proíbe chamada HTTP fora da camada gerada.
- [ ] **T-004** Design tokens compartilhados com o ArchGate.
- [ ] **T-005** Layout base, navegação e rotas tipadas com guards por nível de garantia.
- [ ] **T-006** Seletor de tenant com indicação visual inequívoca e reemissão de token.
- [ ] **T-007** Interceptor de step-up com retomada da operação e preservação de formulário.
- [ ] **T-008** Telas de identidades (usuários, memberships, grupos, contas de serviço).
- [ ] **T-009** Telas de organização (configuração, domínios, políticas).
- [ ] **T-010** Telas de aplicações e clientes OIDC/SAML.
- [ ] **T-011** Telas de provedores de identidade e conectores de diretório.
- [ ] **T-012** Tela de ativos e hierarquia.
- [ ] **T-013** Tela de concessões vigentes com contagem regressiva e revogação.
- [ ] **T-014** Fluxo de solicitação de break-glass com justificativa.
- [ ] **T-015** Fila de aprovação de break-glass.
- [ ] **T-016** Linha do tempo de auditoria com filtros.
- [ ] **T-017** Visão de correlação por `pcid` (ArchGuard + componentes).
- [ ] **T-018** Indicador de integridade da cadeia e acionamento de verificação (L3).
- [ ] **T-019** Exportação assinada da trilha (L3).
- [ ] **T-020** Campanhas de revisão de acesso com origem do acesso e decisão em lote.
- [ ] **T-021** Tela de saúde dos subsistemas (PDP, cofre, auditoria).
- [ ] **T-022** Tela de chaves e rotação (L3).
- [ ] **T-023** i18n pt-BR/en-US e revisão de terminologia.
- [ ] **T-024** CSP, cookies seguros e proteção CSRF.
- [ ] **T-025** Testes E2E dos fluxos privilegiados.
- [ ] **T-026** Remover o console herdado da árvore de build.

## Gate de verificação
E2E verde nos fluxos privilegiados; build falha com cliente defasado; auditoria de
acessibilidade por teclado nos fluxos L3; nenhuma regra de autorização no frontend.
