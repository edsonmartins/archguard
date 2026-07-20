# ADR-0008 — Eliminação da senha-mestra e redesenho do acesso privilegiado (break-glass)

- **Status:** Aceito
- **Data:** 2026-07-19
- **Invariantes tocados:** I-4.1, I-4.2, I-5.1

## Contexto

O upstream oferece dois mecanismos incompatíveis com um produto de PAM:

1. **Master password** — uma senha configurada na organização que permite autenticar como
   **qualquer usuário** daquela organização. A própria documentação do projeto adverte para o
   risco e reconhece que, nos logs básicos, o login por senha-mestra **não é distinguido de um
   login normal**.
2. **Impersonation** administrativa com registro insuficiente para reconstruir *quem*
   realmente executou uma ação.

Em um PAM, esses mecanismos são a definição de backdoor: destroem o não-repúdio e transformam
a trilha em ficção. Nenhuma quantidade de aviso na documentação torna isso aceitável quando o
produto vendido é justamente a garantia de rastreabilidade de acesso privilegiado.

## Decisão

### 1. Remoção total da senha-mestra
O mecanismo é **removido do código**, não desabilitado por configuração (I-4.4 e I-4.1). Não
existe caminho de código que autentique um usuário com credencial que não seja dele. Migrations
removem o campo; teste de invariante impede reintrodução por cherry-pick (ADR-0003).

### 2. Impersonation redesenhada como delegação explícita
Impersonation não é eliminada — é necessária para suporte —, mas passa a ser **delegação
consentida e limitada**:

- **Identidade dupla no token**: o token de impersonation carrega o sujeito impersonado **e**
  o ator real (`act` claim, RFC 8693 — Token Exchange). Toda ação subsequente registra ambos.
- **Consentimento**: por padrão, exige aprovação do usuário-alvo. Modo sem consentimento existe
  apenas via break-glass (item 3).
- **Escopo reduzido**: sessão de impersonation **nunca** herda permissões de administração,
  nunca acessa segredos e nunca aprova solicitações.
- **Tempo-limitada** e revogável, com expiração curta e explícita.
- **Visibilidade**: banner permanente na sessão e **notificação ao usuário-alvo**.

### 3. Break-glass como procedimento formal
Acesso emergencial substitui todo backdoor informal:

- Requer **justificativa obrigatória** vinculada a incidente/chamado.
- Requer **MFA resistente a phishing** do solicitante (step-up).
- Requer **aprovação de N pares** (padrão: 2, configurável por tenant; nunca zero em produção).
- **Janela temporal curta** com expiração automática e revogação de todas as sessões
  derivadas.
- **Alerta em tempo real** para canais de segurança do tenant no momento da solicitação.
- **Fail-closed**: se a auditoria não puder registrar, o break-glass é negado (I-5.4).
- **Revisão obrigatória pós-uso** registrada como artefato do incidente.

### 4. Contas de serviço
Separadas de identidades humanas, sem interface de login interativo, autenticando por
credencial rotacionável custodiada no OpenBao. Uma conta de serviço nunca é impersonada.

## Alternativas consideradas

| Opção | Por que não |
|---|---|
| Manter senha-mestra desabilitada por padrão | Viola I-4.1: o caminho de código continua existindo e pode ser reativado ou explorado |
| Manter impersonation apenas com log melhor | Log melhor não resolve ausência de consentimento nem escopo excessivo |
| Proibir impersonation totalmente | Inviabiliza suporte a cliente; empurraria operadores para soluções piores (compartilhamento de credencial) |

## Consequências

- **Perda deliberada de conveniência operacional.** Suporte precisará de aprovação e
  justificativa. Isso é o produto, não um efeito colateral.
- Divergência estrutural do upstream nos módulos de login (registrada em `DIVERGENCE.md`);
  cherry-picks nessa área exigem revisão manual obrigatória.
- Requisito de canal de notificação em tempo real (e-mail/webhook) no deployment mínimo.
