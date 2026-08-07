# ArchGate service API

O ArchGate resolve o contexto de uma sessão no ArchGuard através de
`POST /api/v1/service/session-context`.

O endpoint aceita somente `Authorization: Bearer $ARCHGUARD_SERVICE_TOKEN`,
configurado exclusivamente no ambiente do processo. O corpo contém apenas o
`subject` opaco do token OIDC:

```json
{"subject":"<oidc-sub>"}
```

A resposta devolve o identificador e o estado da identidade, além dos
`membership_id`, `organization_id` e estados de membership. E-mail, nome,
atributos cifrados e permissões não atravessam essa fronteira. O ArchGate deve
validar o token OIDC (assinatura, issuer, audience, expiração e subject) antes
de chamar o endpoint; o endpoint não é uma API de impersonação.

Sem `ARCHGUARD_SERVICE_TOKEN` o handler permanece fail-closed (401). O segredo
deve ser diferente dos tokens de usuário e ser entregue por secret manager ou
arquivo de bootstrap com permissão 0600.
