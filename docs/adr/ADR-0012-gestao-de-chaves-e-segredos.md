# ADR-0012 — Gestão de chaves e segredos com OpenBao

- **Status:** Aceito
- **Data:** 2026-07-19
- **Invariantes tocados:** I-4.3, I-2.2, I-1.3

## Contexto

O ArchGuard custodia material sensível: chaves de assinatura de JWT (JWKS), chaves de selagem
da trilha de auditoria (ADR-0007), segredos de clientes OAuth dos componentes do ArchGate,
credenciais de conectores de diretório (LDAP/AD) e chaves de criptografia por titular
(crypto-shredding, ADR-0014).

O padrão do upstream — persistir esse material no banco relacional — significa que um dump do
banco entrega tudo. Em um PAM, isso é inaceitável.

O ArchGate já compõe o **OpenBao** (fork MPL-2.0 do Vault, sob a Linux Foundation). Ele já está
no deployment. Nota de licença (I-2.2): MPL-2.0 é **classe condicionada** — permitido
exclusivamente como **serviço em processo separado**, jamais linkado ao binário do ArchGuard.
A integração é por API HTTP.

## Decisão

**O OpenBao é o custodiante de material criptográfico do ArchGuard. O banco armazena
referências (identificadores de chave), nunca valores secretos.**

### Uso por classe de segredo

| Classe | Estratégia |
|---|---|
| **Chaves de assinatura JWT (JWKS)** | Geradas e mantidas no OpenBao; assinatura preferencialmente pelo *transit engine* (a chave privada não deixa o cofre). Rotação com sobreposição e publicação de múltiplas chaves no JWKS |
| **Chave de selagem da auditoria** | Ed25519 no *transit engine*; a aplicação **nunca** obtém a chave privada, apenas solicita assinatura do head da cadeia |
| **Segredos de client OAuth** dos componentes | Armazenados no cofre; o banco guarda referência; rotação sem downtime por convivência de segredos |
| **Credenciais de conectores LDAP/AD** | Cofre, com renovação e revogação |
| **Chaves por titular (LGPD)** | Derivadas/custodiadas no cofre; a eliminação destrói a chave (crypto-shredding) |

### Rotação
- **JWKS**: rotação periódica com sobreposição maior que o TTL máximo de token; chave antiga
  permanece publicada até expirar o último token emitido.
- **Selagem de auditoria**: rotação com registro do intervalo de validade, para que a
  verificação histórica continue possível — a verificação de um selo antigo usa a chave
  pública vigente à época.
- Toda rotação é operação **L3** (ADR-0010) e evento de auditoria.

### Modo degradado (I-1.3, emendado pelo ADR-0017)
O invariante exige **continuidade sob falha** do plano de autenticação. Solução:
- **Cache de curta duração** das chaves públicas e capacidade de assinatura, permitindo
  sobreviver a indisponibilidade transitória do cofre.
- **Perfil `dev`** (fonte normativa: **ADR-0017**): custódia em keystore local selado —
  **explicitamente não suportado em produção**, com aviso de inicialização, marca
  `compliance: non_conformant` no *health check*, negação de operações L3 e recusa de boot
  sob indício de exposição pública.
- Indisponibilidade prolongada do cofre: emissão de novos tokens degrada primeiro; operações
  L3 falham fechado.

## Alternativas consideradas

| Opção | Por que não |
|---|---|
| Chaves no banco (upstream) | Dump do banco compromete tudo; viola I-4.3 |
| HSM/PKCS#11 direto | Melhor garantia, mas custo e complexidade de instalação altos para a base de clientes; **mantido como opção futura**, atrás da mesma interface de custódia |
| Cofre próprio | Reinventar custódia criptográfica é risco desnecessário com OpenBao já no deployment |
| Linkar biblioteca do OpenBao | **Proibido**: MPL-2.0 é classe condicionada, só em processo separado (I-2.2) |

## Consequências

- Acoplamento operacional ao OpenBao no perfil de produção (já presente no ArchGate).
- Interface de custódia (`KeyCustodian`) isola a implementação, viabilizando HSM depois.
- Procedimentos de *disaster recovery* precisam cobrir o cofre: perder as chaves de selagem
  significa perder a **verificabilidade** histórica da trilha (os eventos permanecem, a prova
  não). Isso é requisito explícito de runbook.
