# Eleições de licença — dependências dual-licenciadas

> Normativo (decisão de 2026-07-20, ADR-0002 §3a). Dependência oferecida sob **mais de uma
> licença, à escolha do usuário**, ou reportada como **indeterminada** pelo scanner quando um
> humano determinou a licença real, exige **eleição explícita** registrada aqui. Sem eleição,
> o `license-gate` trata a dependência como licença desconhecida e falha (fail-closed). A
> licença eleita precisa ser **permitida** pela matriz vigente (ADR-0002 / ADR-0019); se a
> única opção permitida não for elegível, a dependência **sai** — não há eleição para licença
> proibida. Toda eleição consta também do `NOTICE`.

## Formato

Uma linha por módulo, em bloco de código: `<módulo> => <SPDX eleita>  # justificativa`. A chave
casa por prefixo com os pacotes do módulo.

## Eleições vigentes

```
(nenhuma eleição vigente)
```

## Eleições pendentes

- `github.com/golang/freetype` (dual **FTL-ou-GPLv2**): eleger **FTL** (permissiva, estilo BSD
  com atribuição) **em T-010**, condicionado à verificação de que a dependência sobrevive ao
  Bloco 3 (só consumidor é captcha/render). Se o consumidor for removido, a dependência sai e
  não há eleição a registrar. Não eleger antes do T-010.
