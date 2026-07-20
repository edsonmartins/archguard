# CONSTITUTION.md — ArchGuard

> **Status:** Vigente (provisória até ratificação do primeiro release GA)
> **Versão:** 1.1.0 *(emenda de I-1.3 por ADR-0017 — ver Anexo B)*
> **Produto:** ArchGuard — plano de controle de identidade da plataforma ArchGate (PAM)
> **Organização:** IntegrAllTech (Integrall.tech) — Maringá/PR, Brasil
> **Base:** fork do Casdoor (Apache License 2.0)

Este documento define os **invariantes** do ArchGuard. Invariantes não são preferências: são
restrições que **nenhum ADR, RFC ou pacote OpenSpec pode violar**. Qualquer proposta que
conflite com um invariante é rejeitada na revisão, ou exige emenda formal desta constituição
(seção 10).

---

## 1. Identidade do produto

**I-1.1** O ArchGuard é o **plano de controle de identidade** do ArchGate. Ele responde por
autenticação (AuthN), gestão do ciclo de vida de identidades, emissão de tokens e federação.
Ele **não** é o plano de dados: gravação de sessão, proxy de protocolo e brokering de
credencial pertencem a Warpgate, Apache Guacamole, NetBird e OpenBao.

**I-1.2** O ArchGuard é um **produto derivado com governança própria**, não um wrapper. Toda
decisão de arquitetura é tomada pela IntegrAllTech; o upstream é fonte de correções, não de
direção de produto.

**I-1.3** *(emendado por ADR-0017 em 2026-07-20)* O ArchGuard é **autossuficiente em
continuidade de runtime**: a indisponibilidade transitória de qualquer serviço adjacente
(OpenBao, OpenFGA, coletor OTLP) **não derruba** o plano de autenticação nem invalida sessões
existentes. O perfil `dev` (ArchGuard + PostgreSQL) autentica, emite tokens OIDC e audita sem
serviço externo, para fins de desenvolvimento, CI e demonstração. **A configuração suportada em
produção é o perfil `production`**, no qual a custódia de chaves em OpenBao é obrigatória
(I-4.3). Autossuficiência descreve o comportamento sob falha, não a configuração comercialmente
suportada. Perfis normativos em ADR-0017.

---

## 2. Licenciamento e propriedade intelectual

**I-2.1** A base forkada é **Apache License 2.0**. O arquivo `LICENSE` do upstream e o
`NOTICE` de atribuição são **preservados integralmente e para sempre**, com registro explícito
do commit-base do fork.

**I-2.2** *(emendado por ADR-0019 em 2026-07-20)* Dependências sob **copyleft por arquivo**
(MPL-2.0, EPL-2.0, CDDL) são permitidas **linkadas ao binário quando consumidas sem
modificação**, sujeitas às obrigações de aviso, atribuição e disponibilização de fonte dos
arquivos cobertos. Dependência sob copyleft por arquivo que seja **modificada, vendorizada com
patch ou forkada** só é permitida como **serviço externo em processo separado** — caso
contrário, os arquivos modificados tornam-se publicáveis sob a licença original, incompatível
com o produto proprietário. Nenhuma dependência sob **AGPL, GPL, LGPL linkada, SSPL, BUSL ou
licença "community" com restrição por porte de empresa** entra na árvore de build, em qualquer
circunstância.

**I-2.3** Toda dependência nova exige verificação de licença registrada no PR. O CI **quebra o
build** diante de licença não aprovada (SBOM + license gate obrigatórios).

**I-2.4** Código proveniente de forks, trechos de terceiros ou geração assistida por IA que não
possa ter procedência licencial demonstrada **não é incorporado**.

---

## 3. Soberania de dados e LGPD

**I-3.1** **Deploy on-premises / self-hosted é cenário de primeira classe.** Nenhuma
funcionalidade essencial pode depender de serviço SaaS de terceiro, telemetria externa ou
chamada de rede para fora do perímetro do cliente.

**I-3.2** Nenhum dado pessoal, credencial, segredo ou evento de auditoria trafega para fora do
perímetro definido pelo cliente. Telemetria de produto é **opt-in explícito** e anonimizada.

**I-3.3** Dados pessoais têm **base legal, finalidade e prazo de retenção declarados** no
modelo de dados. Campos sem classificação LGPD não são aceitos em migrations.

**I-3.4** O direito à eliminação (LGPD Art. 18, VI) é atendido por **crypto-shredding e
pseudonimização**, jamais por deleção de registros da trilha de auditoria (ver I-5.3).

---

## 4. Segurança como invariante

**I-4.1** **Não existe backdoor administrativo.** Nenhum mecanismo de senha-mestra,
credencial global, conta oculta ou bypass de MFA existe no produto. Acesso emergencial
ocorre exclusivamente pelo procedimento de **break-glass auditado** (RFC-0003 / ADR-0008).

**I-4.2** **MFA é obrigatório para toda identidade com privilégio.** Operações privilegiadas
exigem **step-up authentication** com fator resistente a phishing (WebAuthn/passkey) — TOTP é
fallback, nunca o padrão para privilégio.

**I-4.3** **Segredos nunca são persistidos em claro.** Chaves de assinatura, segredos de
cliente OAuth e material criptográfico residem no OpenBao ou em KMS equivalente; o banco
armazena referências, não valores.

**I-4.4** **Default seguro.** Toda opção de configuração cujo valor inseguro seja possível
nasce desligada. Nenhuma instalação sobe com credencial padrão utilizável.

**I-4.5** Todo token emitido é **auditável, revogável e de escopo mínimo**. Não existe token
de vida longa sem revogação possível.

---

## 5. Auditoria

**I-5.1** Todo evento de autenticação, autorização, mutação administrativa e acesso
privilegiado gera **registro de auditoria estruturado**.

**I-5.2** A trilha de auditoria é **append-only e tamper-evident**: encadeamento por hash e
selagem periódica assinada. Não existe caminho de código — nem administrativo, nem de
suporte — que edite ou remova um evento já selado.

**I-5.3** Conflito entre eliminação LGPD e imutabilidade da trilha resolve-se por
**pseudonimização do sujeito**, preservando a integridade da cadeia (ADR-0014).

**I-5.4** **Falha de auditoria é falha da operação.** Se o evento não pode ser persistido de
forma durável, a operação privilegiada correspondente é negada (*fail-closed*).

---

## 6. Multi-tenancy

**I-6.1** A **organização (tenant)** é a fronteira primária de isolamento. Toda entidade de
domínio carrega tenant e todo acesso a dados é filtrado por tenant na camada de repositório.

**I-6.2** Um **usuário pode pertencer a múltiplas organizações**. A relação usuário↔organização
é entidade explícita (`membership`) com papéis próprios por organização.

**I-6.3** Cruzamento de fronteira entre tenants é **impossível por construção**, não por
disciplina de código. Toda query sem predicado de tenant é bloqueada por teste automatizado.

---

## 7. Arquitetura e stack

**I-7.1** **PostgreSQL 15+ é o único banco suportado.** Multi-database do upstream é removido.

**I-7.2** O **core de identidade permanece em Go** (herança do fork). Serviços adjacentes da
IntegrAllTech seguem **Java 21 + Spring Boot 3.x**, integrando-se ao ArchGuard exclusivamente
por **OIDC/OAuth2 e API REST versionada** — nunca por acesso direto ao banco.

**I-7.3** O **console administrativo é React 19 + TypeScript + Mantine v9 + Archbase**,
desacoplado do backend e consumindo apenas a API pública versionada. O console é substituível
sem alterar o core.

**I-7.4** **AuthN e AuthZ são subsistemas separados.** Autorização de granularidade fina é
delegada a um PDP externo (OpenFGA). O core não embute regras de negócio de autorização de
sessão privilegiada.

**I-7.5** Deployment padrão é **Docker Swarm + Traefik**; observabilidade padrão é
**OpenTelemetry → VictoriaMetrics + Grafana + Loki + Tempo**.

**I-7.6** **A API pública é contrato.** Toda mudança quebra-compatibilidade exige nova versão
de rota e período de convivência declarado.

---

## 8. Governança do fork

**I-8.1** O fork mantém **fork point registrado** e rastreabilidade de divergência. Sincronismo
com upstream é **seletivo por cherry-pick**, jamais merge cego de branch.

**I-8.2** Correções de **segurança** do upstream são triadas em até **72 horas** da divulgação.

**I-8.3** Funcionalidades do upstream fora do escopo de PAM (integrações AI/MCP, provedores
de terceiros irrelevantes ao mercado brasileiro) são **removidas ou desabilitadas**, reduzindo
superfície de ataque e custo de manutenção.

**I-8.4** Divergência é intencional e documentada: todo desvio estrutural do upstream tem ADR.

---

## 9. Método de engenharia

**I-9.1** **Governança antes de código.** A ordem é: CONSTITUTION → ADR → RFC → pacote
OpenSpec → implementação.

**I-9.2** Todo pacote OpenSpec contém `proposal.md`, `design.md`, `tasks.md` e
`specs/<capability>/spec.md` com critérios de aceite em **WHEN/THEN**.

**I-9.3** **Granularidade de sessão:** toda tarefa cabe em uma sessão de trabalho de agente,
com critério de pronto verificável.

**I-9.4** **"Pass 5 é a única autoridade sobre 'pronto'."** Nenhuma tarefa é concluída sem
passar pelo gate de verificação definido no pacote.

**I-9.5** Toda especificação afirma-se contra **fonte primária** (RFC do IETF, documentação
oficial, código do upstream). Imprecisão contra fonte primária é defeito, não estilo.

---

## 10. Emenda

**I-10.1** Emendar esta constituição exige ADR dedicado com: invariante afetado, justificativa,
impacto sobre pacotes OpenSpec vigentes e plano de migração.

**I-10.2** Invariantes das seções **2 (licença), 3 (soberania) e 4 (segurança)** são
**pétreos na vigência da v1**: violá-los exige nova major version do produto e aprovação dos
sócios-fundadores.

---

## Anexo A — Índice normativo

| Documento | Papel |
|---|---|
| `docs/adr/*` | Decisões arquiteturais irreversíveis ou caras de reverter |
| `docs/rfc/*` | Desenhos técnicos detalhados, contratos e modelos |
| `openspec/changes/*` | Pacotes de mudança executáveis com critérios WHEN/THEN |
| `LICENSE` / `NOTICE` | Obrigações Apache 2.0 do fork |

## Anexo B — Registro de emendas

| Data | Invariante | ADR | Motivo |
|---|---|---|---|
| 2026-07-20 | I-1.3 | ADR-0017 | Contradição com I-4.3 e RFC-0001: autossuficiência de runtime confundida com configuração suportada em produção. Invariante pétreo I-4.3 preservado sem alteração. |
| 2026-07-20 | I-2.2 | ADR-0019 | Correção de erro de redação: MPL é copyleft por ARQUIVO, não por linkagem. MPL/EPL/CDDL não modificado passa a ser permitido linkado (obrigações de aviso/fonte); modificado só em processo separado. GPL/AGPL/LGPL-linkada/SSPL/BUSL seguem proibidas. **Ratificado pelos sócios-fundadores** (I-10.2). |
