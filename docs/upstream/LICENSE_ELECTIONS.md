# Eleições de licença — dependências dual-licenciadas

> Normativo (decisão de 2026-07-20, ADR-0002 §3a). Dependência oferecida sob **mais de uma
> licença, à escolha do usuário**, exige **eleição explícita** da licença adotada, registrada
> aqui. Sem eleição registrada, o `license-gate` trata a dependência como licença desconhecida
> e falha (fail-closed). A licença eleita deve constar também do `NOTICE` e do SBOM.

## Formato

Uma linha por módulo: `<módulo> => <SPDX eleita> (# justificativa)`. A licença eleita **precisa
ser permitida** pela matriz vigente (ADR-0002 / ADR-0019); se a única opção permitida não for
elegível, a dependência **sai** — não há eleição para licença proibida.

## Eleições vigentes

```
github.com/golang/freetype => FTL   # dual FTL-ou-GPLv2; elegemos FTL (permissiva, estilo BSD com atribuição). GPLv2 seria proibida.
```

> `github.com/golang/freetype` sobrevive ao Bloco 3? Verificar em T-010: se o único consumidor
> (captcha/render) for removido, a dependência sai e esta eleição é retirada. A eleição vale
> enquanto o módulo estiver na árvore de build.
