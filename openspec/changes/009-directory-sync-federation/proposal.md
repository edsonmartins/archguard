# Proposal — 009 · Sincronismo com diretórios e federação de entrada

## Por quê

Clientes brasileiros operam AD/LDAP corporativo e, cada vez mais, IdPs corporativos
(Entra ID, Google Workspace, Okta). Sem sincronismo confiável, o desligamento de um colaborador
não se reflete no PAM — que é a falha de segurança mais comum em gestão de acesso privilegiado.
Além disso, SCIM de entrada é justamente a lacuna que inviabilizou o PoC anterior; aqui é
requisito explícito com testes (RFC-0007).

## O que muda

- Conector LDAP/AD com sincronização incremental de usuários e grupos, por organização.
- SCIM 2.0 **de entrada** (ArchGuard como alvo de provisionamento).
- Federação de login com IdPs corporativos (SAML 2.0 / OIDC) e *JIT provisioning* que cria
  **membership**, nunca identidade duplicada.
- Regra dura: **step-up L3 nunca é delegado** ao IdP externo.
- LDAP e RADIUS embutidos mantidos como compatibilidade de borda, desabilitados por padrão e
  proibidos para acesso privilegiado.

## Impacto

- **Depende de:** 002, 003, 005, 006.
- **Risco:** qualidade dos dados de diretório do cliente; mapeamentos ambíguos de grupo.
