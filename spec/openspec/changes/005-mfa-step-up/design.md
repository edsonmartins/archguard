# Design — 005 · MFA obrigatório e step-up

Base normativa: ADR-0010; contrato de claims em RFC-0006.

## Níveis de garantia

| Nível | Exigência | Frescor |
|---|---|---|
| L1 | Sessão válida | — |
| L2 | Fator forte na sessão | Janela moderada |
| L3 | **Reautenticação WebAuthn imediata** | Janela curta |

O nível obtido é expresso em `acr`; os métodos, em `amr`; `auth_time` sustenta o cálculo de
frescor. A avaliação de frescor ocorre **no momento da operação**, não no login.

## Classificação de operações

Toda operação da API declara seu nível exigido em metadado. **Operação sem classificação não
sobe** (mesma regra do inventário de endpoints, ADR-0015). O middleware de autorização compara
exigido × obtido e responde com erro específico de garantia insuficiente, informando o `acr`
necessário — permitindo ao console disparar o step-up e repetir a operação (RFC-0005, §6).

Operações L3 (mínimo): abrir sessão privilegiada, aprovar break-glass, rotacionar chave,
exportar auditoria, verificar cadeia, eliminar dados de titular, remover fator forte, alterar
política de MFA, criar conta de serviço.

## Fatores

- **WebAuthn**: padrão. RP ID estável, atestação conforme política, suporte a múltiplos
  autenticadores por identidade.
- **TOTP**: fallback. Válido para L2 em tenants que o permitam; **nunca** para L3.
- **Códigos de recuperação**: uso único, exibidos uma vez; regeneração invalida todos.
- **SMS/e-mail**: não suportados como fator para privilégio.

## Enrolamento obrigatório

Identidade com papel privilegiado e sem fator forte entra em estado `enrollment_required`: a
sessão só permite registrar fator. Não há "lembrar depois".

## Recuperação sem backdoor

Perda de dispositivo: se não houver segundo fator nem código de recuperação, a recuperação
segue o **mesmo rigor do break-glass** — justificativa, aprovação de pares, auditoria,
notificação. Não existe reset administrativo silencioso (I-4.1).

## Anti-abuso

Limitação de taxa por identidade, IP e dispositivo; bloqueio progressivo; detecção de padrões
de credential stuffing; **todo** evento de MFA (sucesso, falha, enrolamento, remoção) auditado.
Remoção de fator forte é L3 e notifica a identidade.
