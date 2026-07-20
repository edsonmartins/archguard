# RFC-0007 — Migração, coexistência e sincronismo com diretórios

- **Status:** Proposto
- **Data:** 2026-07-19
- **ADRs relacionados:** ADR-0001, ADR-0006, ADR-0011, ADR-0015

## 1. Objetivo

Definir como sair do PoC baseado em Kanidm, como sincronizar identidades com diretórios
corporativos (AD/LDAP, IdPs SAML/OIDC, SCIM) e como conduzir a virada de cada componente do
ArchGate sem janela de indisponibilidade prolongada.

## 2. Ponto de partida

O PoC sobre Kanidm é **descontinuado** (ADR-0001). Ele não é migrado: é **substituído**. O que
se aproveita é conhecimento de domínio e o inventário de identidades, não código nem esquema.

## 3. Estratégia de virada: *strangler* por componente

A virada é **por componente do ArchGate**, nunca "tudo de uma vez":

```
Fase A  Warpgate            (menor blast radius, público técnico)
Fase B  Apache Guacamole
Fase C  NetBird
Fase D  OpenBao             (maior criticidade — por último)
Fase E  Proxy Oracle JDBC e produtos IntegrAllTech
```

Para cada fase:
1. Registrar o cliente OIDC no ArchGuard (RFC-0006).
2. Habilitar autenticação dupla (mecanismo antigo + ArchGuard) em janela controlada.
3. Migrar coorte-piloto de operadores; validar auditoria correlacionada (`pcid`).
4. Executar a suíte de conformidade do componente (RFC-0006, §8).
5. Desabilitar o mecanismo antigo; manter rollback documentado por período definido.
6. **Ponto de não retorno explícito**: após o desligamento, o rollback exige procedimento
   formal — não é decisão de plantão.

## 4. Importação de identidades

**Fontes:** exportação do PoC, AD/LDAP do cliente, planilhas de operadores (fase inicial).

**Regras:**
- **Nenhuma senha é importada.** Toda identidade importada entra em estado de enrolamento:
  primeiro acesso exige verificação e registro de fator forte (ADR-0010).
- Deduplicação por `email_hash` com relatório de conflito; **fusão automática silenciosa é
  proibida** (RFC-0002, §6).
- Importação gera **membership**, não identidade duplicada, quando a pessoa já existe.
- Toda importação é evento de auditoria com lote identificável e reversível por revogação de
  memberships criados.

## 5. Sincronismo com diretórios

### 5.1 LDAP/AD (conector de saída — leitura do diretório do cliente)
- Sincronização incremental de usuários e grupos por organização.
- **Mapeamento explícito** diretório→ArchGuard, versionado: atributos, grupos, filtro de
  escopo (nunca "toda a árvore").
- Desativação no diretório ⇒ **suspensão do membership** correspondente (não deleção).
- Credenciais do conector no OpenBao (ADR-0012).
- Conflito de precedência declarado: para identidades sincronizadas, o diretório é
  autoritativo para atributos e pertencimento a grupos; papéis e concessões privilegiadas são
  **sempre** do ArchGuard.

### 5.2 SCIM 2.0
- **Entrada** (o ArchGuard como alvo de provisionamento de um IdP do cliente): suportado.
- **Saída** (o ArchGuard provisionando terceiros): fora do escopo v1.
- Esta é exatamente a lacuna que inviabilizou o Kanidm — aqui é requisito explícito de
  produto, com testes.

### 5.3 Federação (SAML/OIDC)
- Login federado com o IdP corporativo do cliente é suportado (ADR-0015, catálogo curado).
- **O ArchGuard não delega step-up L3**: mesmo com federação, operações L3 exigem fator forte
  verificado pelo ArchGuard (ADR-0010). Confiar cegamente no `acr` de terceiro anularia o
  controle sobre acesso privilegiado.
- *Just-in-time provisioning* cria membership, jamais identidade duplicada.

## 6. Servidor LDAP e RADIUS embutidos

Mantidos do upstream (ADR-0015) como **compatibilidade de borda** para equipamentos e sistemas
legados que não falam OIDC. Restrições normativas:
- Escopo mínimo, desabilitados por padrão (I-4.4).
- **Nunca** como caminho para operação privilegiada L3 — não carregam `acr` nem correlação.
- Todo acesso por esses protocolos é auditado e sinalizado como canal legado.

## 7. Plano de rollback

| Fase | Rollback | Ponto de não retorno |
|---|---|---|
| Importação de identidades | Revogar memberships do lote | Após primeiro login com fator registrado |
| Virada de componente | Reabilitar mecanismo antigo | Após desligamento formal do mecanismo antigo |
| Migração de esquema (RFC-0002) | Restaurar backup verificado | Após fusão de identidades duplicadas |

Todo rollback exige backup verificado **antes** da execução e ensaio prévio em cópia de
produção.

## 8. Critérios de sucesso da migração

1. Nenhuma identidade humana com contas duplicadas entre tenants.
2. 100% das identidades privilegiadas com fator forte registrado.
3. Auditoria correlacionada ponta a ponta (`pcid`) verificável em todos os componentes.
4. Zero acesso privilegiado por canal legado (LDAP/RADIUS).
5. Suíte de conformidade verde para todos os componentes.

## 9. Questões em aberto

- Coexistência temporária de dois IdPs por componente é aceitável operacionalmente, ou a
  virada deve ser atômica por componente?
- Volume e qualidade dos dados de identidade do PoC — inventário pendente.
- Clientes com AD legado sem HTTPS interno: impacto no RP ID do WebAuthn (ADR-0010).
