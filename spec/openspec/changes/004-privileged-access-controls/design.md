# Design — 004 · Controles de acesso privilegiado

## Delegação (impersonation)

Token de delegação carrega o sujeito impersonado em `sub` e o ator real em `act`
(RFC 8693). Toda ação subsequente registra **ambos** na auditoria.

Restrições impostas por construção:
- **NUNCA** herda permissões administrativas;
- **NUNCA** acessa segredos ou o cofre;
- **NUNCA** aprova solicitações (inclusive break-glass);
- expiração curta e revogação imediata disponível ao alvo e ao administrador;
- banner permanente na sessão + notificação ao usuário-alvo.

Consentimento é o padrão. Delegação sem consentimento existe apenas dentro de break-glass.

## Break-glass

Máquina de estados:

```
solicitado ──step-up OK──► aguardando_aprovacao ──N aprovações──► ativo
     │                              │                              │
     └─ negado                      └─ expirado/rejeitado          └─ expirado
                                                                    └─ revogado
```

Regras normativas:
- justificativa obrigatória vinculada a incidente/chamado;
- step-up com fator resistente a phishing (WebAuthn) — TOTP **não** aceito;
- N aprovadores distintos do solicitante (padrão 2; **zero é proibido em produção**);
- aprovador não pode ser sessão de delegação;
- alerta em tempo real no momento da **solicitação** (não da aprovação);
- janela curta com expiração automática e revogação em cascata das sessões derivadas;
- **fail-closed**: sem auditoria durável ou sem canal de notificação, a solicitação é negada;
- revisão pós-uso obrigatória, registrada como artefato.

## Concessões

`privileged_grant`: sujeito (membership), alvo (ativo/escopo), janela temporal, origem
(`normal` | `breakglass`), aprovações, status. Expiração é avaliada **no momento da decisão**,
não apenas por job — o job apenas materializa a limpeza.

Concessões são projetadas como relação no PDP (pacote 007), de modo que a expiração se reflita
no grafo de autorização.

## Contas de serviço

Tipo `service` na identidade: sem interface de login interativo, credencial rotacionável
custodiada no cofre, nunca impersonável, com auditoria de uso.
