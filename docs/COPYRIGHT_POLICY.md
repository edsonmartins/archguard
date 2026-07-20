# Política de cabeçalhos de copyright — ArchGuard

> Normativa a partir de 2026-07-20 (T-005). Fundamento: ADR-0002 (obrigações Apache 2.0 do
> fork) e I-2.1. Aplica-se a todo arquivo de código-fonte (`.go`, `.ts`, `.tsx`, `.js`,
> `.sql`, `.sh`, `.py` e equivalentes). Não se aplica a documentação Markdown, configuração
> declarativa e assets.

## 1. Arquivos herdados do upstream, não modificados

Cabeçalho original preservado **sem qualquer alteração**.

## 2. Arquivos herdados do upstream, modificados pelo ArchGuard

O cabeçalho original ("Copyright ... The Casdoor Authors ...") é preservado integralmente —
**nunca removido nem reescrito** — e recebe, imediatamente após o bloco original, a linha de
declaração de modificação:

```go
// Copyright 2021 The Casdoor Authors. All Rights Reserved.
// [... bloco original intacto ...]
// limitations under the License.
//
// Modified by IntegrAllTech Ltda. — changes recorded in docs/upstream/DIVERGENCE.md.
```

A linha é adicionada **uma única vez** (na primeira modificação), sem data por arquivo — o
histórico de modificações é do git e do `DIVERGENCE.md`, não do cabeçalho.

## 3. Arquivos novos, criados pelo ArchGuard

```go
// Copyright 2026 IntegrAllTech Ltda.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
```

O ano é o da criação do arquivo e **não é atualizado** anualmente. A sintaxe de comentário
adapta-se à linguagem (`--` em SQL, `#` em shell, `/* */` em CSS).

> Nota: o licenciamento **externo** do produto ArchGuard (proprietário vs. Apache 2.0 de
> arquivos derivados) é matéria da due diligence jurídica pré-M1 (ADR-0002). Esta política
> governa apenas cabeçalhos e atribuição; na dúvida, preservar sempre é o default seguro.

## 4. Proibições

- Remover, truncar ou "atualizar" cabeçalho de copyright de terceiros (ADR-0002, viola
  Apache 2.0 §4).
- Usar a marca do upstream em cabeçalho de arquivo novo.
- Cabeçalho gerado com ano futuro, autor genérico ("Contributors") ou empresa errada.

## 5. Verificação

A verificação automatizada de cabeçalho em arquivos novos entra no CI em T-019. Até lá, a
conferência é manual na revisão de cada commit.
