# ADR-0010 — MFA obrigatório, WebAuthn-first e step-up authentication

- **Status:** Aceito
- **Data:** 2026-07-19
- **Invariantes tocados:** I-4.2, I-4.4, I-6.2

## Contexto

O upstream suporta WebAuthn/passkeys e TOTP, mas com política de aplicação frouxa: MFA é
majoritariamente opção do usuário. Em PAM, o inverso é obrigatório — o fator forte é o
mecanismo que impede que credencial vazada vire acesso privilegiado a produção.

Além disso, autenticação binária (autenticado / não autenticado) é insuficiente: listar os
próprios acessos e abrir uma sessão root em banco de produção não podem exigir a mesma prova.

## Decisão

### 1. MFA obrigatório para privilégio
Toda identidade com qualquer papel privilegiado **deve** ter ao menos um fator forte
registrado. Sem fator registrado, o login resulta em **estado de enrolamento obrigatório** —
não em acesso concedido.

### 2. WebAuthn-first
- **WebAuthn/passkey é o fator padrão e recomendado** (resistente a phishing).
- **TOTP é fallback explícito**, permitido para acesso comum, **proibido como único fator para
  operações privilegiadas** e para break-glass (ADR-0008).
- **SMS e e-mail como fator não são suportados** para privilégio.
- **Códigos de recuperação** de uso único, gerados uma vez, exibidos uma vez, com invalidação
  em massa na regeneração.

### 3. Step-up authentication (AAL escalonado)
Operações são classificadas por nível de garantia exigido:

| Nível | Exemplos | Exigência |
|---|---|---|
| **L1** | Consultar próprio perfil, listar sessões | Sessão válida |
| **L2** | Alterar dados, gerenciar usuários do tenant | Fator forte na sessão, dentro de janela de frescor |
| **L3** | Abrir sessão privilegiada, aprovar break-glass, rotacionar chave, exportar auditoria | **Reautenticação WebAuthn imediata** (frescor curto), independentemente da idade da sessão |

O nível exigido e o obtido são expressos em claims (`acr`/`amr`) e propagados aos componentes
do ArchGate (RFC-0006). Um componente **pode e deve** recusar token cujo `acr` seja inferior
ao exigido pela operação.

### 4. Política por tenant, mais restritiva vence
A política de MFA é definida por organização. Para identidade com múltiplos memberships
(ADR-0006), a exigência avaliada é a do **tenant ativo**; a troca de tenant pode disparar
step-up.

### 5. Anti-abuso
Limitação de taxa por identidade, IP e dispositivo; bloqueio progressivo; detecção de
*credential stuffing*; **todo evento de MFA (sucesso, falha, enrolamento, remoção de fator)
é auditado** (I-5.1). Remoção de fator forte é operação L3.

## Alternativas consideradas

| Opção | Por que não |
|---|---|
| MFA opcional com nudge | Adoção real fica abaixo do necessário; incompatível com I-4.2 |
| TOTP como padrão | Vulnerável a phishing em tempo real — exatamente o vetor que compromete PAM |
| MFA único no login, sem step-up | Sessão longa vira credencial permanente de alto privilégio |
| Delegar MFA integralmente ao IdP corporativo do cliente | Suportado como federação, mas **não pode ser a única garantia**: o ArchGuard precisa impor step-up próprio nas operações L3 (ver RFC-0006) |

## Consequências

- Fricção deliberada nas operações críticas.
- Exigência de HTTPS e domínio estável (RP ID do WebAuthn) — impacta topologia de deployment
  e é requisito documentado de instalação.
- Cenários de perda de dispositivo exigem processo de recuperação auditado que **não** pode
  virar backdoor (I-4.1): recuperação passa por aprovação de pares, como o break-glass.
