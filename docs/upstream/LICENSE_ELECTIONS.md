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
github.com/golang/freetype => FTL  # dual FTL-ou-GPLv2; via fogleman/gg em object/user_avatar_identicon.go (avatar identicon, feature geral que sobrevive ao Bloco 3). FTL é permissiva (estilo BSD com atribuição). Eleito em T-010.
```
