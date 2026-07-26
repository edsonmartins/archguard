# RFC-0005 — ArchGuard Console: arquitetura frontend

- **Status:** Diferido (referência da opção greenfield adiada pelo [ADR-0020](../adr/ADR-0020-evolucao-do-console-herdado.md), 2026-07-26)
- **Data:** 2026-07-19
- **ADRs relacionados:** ADR-0004 (superado), ADR-0006, ADR-0007, ADR-0008, ADR-0010, ADR-0020

> ⚠️ Esta RFC descreve a arquitetura frontend **greenfield** (React 19 + Mantine v9 + Archbase).
> O ADR-0020 diferiu esse rewrite e adotou a **evolução do console herdado**. Os requisitos
> comportamentais (contrato, tenant, step-up, segurança, i18n) permanecem válidos; o *stack* de
> implementação é o do console herdado até uma eventual reavaliação pós-piloto.

## 1. Objetivo

Especificar a arquitetura do console administrativo em React 19 + TypeScript + Mantine v9 +
Archbase: estrutura, contrato com o backend, modelo de navegação orientado a PAM e requisitos
de segurança do cliente.

## 2. Princípios

1. **Contrato primeiro.** O OpenAPI do backend é a fonte da verdade; o cliente TypeScript é
   **gerado**, nunca escrito à mão. Build falha se o cliente estiver defasado.
2. **Sem endpoint de UI.** Nada é criado "só para a tela" (I-7.6).
3. **Console é substituível.** Nenhuma regra de negócio de autorização vive no frontend —
   ele exibe o que a API autoriza. Esconder botão não é controle de acesso.
4. **Domínio de PAM, não CRUD de IAM.** A navegação reflete o raciocínio do operador.

## 3. Stack

| Camada | Escolha |
|---|---|
| Framework | React 19 + TypeScript (strict) |
| UI | Mantine v9 + **Archbase** (componentes e padrões da casa) |
| Estado de servidor | TanStack Query |
| Estado local | Hooks/contexto; **sem** store global para dados remotos |
| Roteamento | Roteador com rotas tipadas e *guards* por nível de garantia |
| Formulários | Padrão Archbase, com validação derivada do contrato |
| i18n | pt-BR primário, en-US secundário |
| Build | Vite |
| Testes | Unitários + testes de fluxo (E2E) para caminhos privilegiados |

## 4. Estrutura de navegação (v1)

```
Visão geral do tenant
├── Identidades
│   ├── Usuários (identidade global + memberships)
│   ├── Grupos
│   └── Contas de serviço
├── Organizações
│   ├── Configuração e domínios verificados
│   ├── Memberships e convites
│   └── Políticas (MFA, break-glass, retenção)
├── Acesso privilegiado
│   ├── Ativos e hierarquia
│   ├── Concessões vigentes
│   ├── Solicitações de break-glass  (fila de aprovação)
│   └── Campanhas de revisão de acesso
├── Aplicações e federação
│   ├── Clientes OIDC/SAML (Warpgate, Guacamole, NetBird, OpenBao…)
│   ├── Provedores de identidade
│   └── Conectores de diretório e sincronismos
├── Auditoria
│   ├── Linha do tempo (com correlação de sessão privilegiada)
│   ├── Verificação de integridade da cadeia
│   └── Exportação assinada
└── Operação
    ├── Saúde dos subsistemas (PDP, cofre, auditoria)
    └── Chaves e rotação
```

## 5. Seletor de tenant

- Componente permanente no cabeçalho, refletindo os memberships da identidade (ADR-0006).
- Troca de tenant **recarrega o contexto e obtém novo token** — nunca reaproveita token de
  outro tenant.
- Troca é operação auditada; pode **disparar step-up** se a política do tenant destino for
  mais restritiva (ADR-0010).
- Indicação visual inequívoca do tenant ativo (cor/etiqueta): operar no tenant errado é a
  classe de erro humano mais cara em PAM.

## 6. Step-up no cliente

- Resposta da API sinaliza **garantia insuficiente** com erro específico e o `acr` exigido.
- Interceptor global captura, apresenta o desafio WebAuthn e **repete a operação** após o
  step-up — sem perder o estado do formulário.
- Operações L3 exibem confirmação explícita descrevendo o efeito (ex.: "isto eliminará
  permanentemente os dados pessoais do titular", ADR-0014).

## 7. Telas de alta criticidade

**Auditoria.** Linha do tempo por tenant com filtros por ator, ação, alvo, resultado e
período; visão de correlação que reúne autenticação (ArchGuard) + sessão (Warpgate/Guacamole)
+ acesso a segredo (OpenBao) sob o mesmo identificador; **indicador de integridade da cadeia**
sempre visível (íntegra / divergente / não verificada). Divergência é apresentada com destaque
máximo, jamais como aviso discreto.

**Break-glass.** Solicitação com justificativa obrigatória vinculada a incidente; fila de
aprovação com identificação clara do solicitante, do alvo e do prazo; contagem regressiva
visível na concessão vigente; revogação imediata em um clique.

**Revisão de acesso.** Campanha por ativo ou por membership, com resposta do PDP sobre acesso
efetivo (RFC-0004) e ações de manutenção/revogação em lote — cada decisão auditada.

## 8. Segurança do cliente

- Tokens **não** em `localStorage`: sessão por cookie `HttpOnly` + `Secure` + `SameSite`, com
  proteção CSRF; token de acesso mantido apenas em memória quando necessário.
- CSP restritiva, sem `eval`, sem script inline.
- Nenhum segredo embutido no bundle.
- Logout propaga back-channel (ADR-0011).
- Inatividade encerra sessão conforme política do tenant.

## 9. Padrões de UX exigidos

- **Agregados honestos com detalhe sob demanda**: toda superfície de resumo carrega sinal de
  severidade suficiente para indicar se o *drill-down* é necessário. Um contador verde não
  pode esconder negativas no detalhe — se há divergência, falha de sincronização do PDP ou
  break-glass pendente, o topo mostra.
- Estado vazio, de carregamento e de erro definidos para toda tela.
- Ações destrutivas exigem confirmação que descreve a consequência, não apenas "Confirmar".

## 10. Questões em aberto

- Componentes de visualização de grafo de autorização (para "por que este acesso foi
  concedido") — construir sobre Archbase ou avaliar biblioteca dedicada?
- Estratégia de *feature flags* no console (avaliar UseDevKit).
