# Tasks — 009 · Sincronismo e federação de entrada

- [x] **T-001** Modelar `directory_connector` por organização com mapeamento versionado.
- [ ] **T-002** Implementar conector LDAP/AD com sincronização incremental.
- [ ] **T-003** Exigir filtro de escopo obrigatório na configuração do conector.
- [ ] **T-004** Implementar mapeamento de atributos e grupos com validação.
- [ ] **T-005** Implementar suspensão de membership na desativação no diretório.
- [ ] **T-006** Custodiar credenciais do conector no cofre.
- [ ] **T-007** Implementar SCIM 2.0 de entrada (usuários).
- [ ] **T-008** Implementar SCIM 2.0 de entrada (grupos).
- [ ] **T-009** Integrar SCIM à deduplicação por `email_hash`.
- [ ] **T-010** Implementar federação SAML 2.0 de entrada.
- [ ] **T-011** Implementar federação OIDC de entrada.
- [ ] **T-012** Implementar JIT provisioning que cria membership, não identidade duplicada.
- [ ] **T-013** Garantir que step-up L3 nunca seja satisfeito por `acr` de terceiro.
- [ ] **T-014** Desabilitar LDAP/RADIUS embutidos por padrão e restringir escopo.
- [ ] **T-015** Bloquear operações L3 originadas de canais legados.
- [ ] **T-016** Auditar todos os eventos de sincronismo, federação e canal legado.
- [ ] **T-017** Ferramenta de importação com estado `enrollment_required`.
- [ ] **T-018** Relatório de conflito de deduplicação e fluxo de fusão assistida.
- [ ] **T-019** Teste: desativação no diretório suspende o membership em uma execução.
- [ ] **T-020** Teste: JIT com e-mail conhecido não cria segunda identidade.
- [ ] **T-021** Teste: papel privilegiado não é concedido automaticamente por grupo de
      diretório sem mapeamento aprovado.

## Gate de verificação
Ciclo completo de provisionamento e desprovisionamento validado contra AD de laboratório;
nenhuma identidade duplicada gerada por federação; L3 inacessível por canal legado ou por
`acr` de terceiro.
