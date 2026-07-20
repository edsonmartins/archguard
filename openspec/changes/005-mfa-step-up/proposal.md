# Proposal — 005 · MFA obrigatório e step-up authentication

## Por quê

Autenticação binária é insuficiente para PAM: listar os próprios acessos e abrir sessão root em
produção não podem exigir a mesma prova. Além disso, MFA opcional produz adoção real abaixo do
necessário, e TOTP como fator principal é vulnerável ao phishing em tempo real — exatamente o
vetor que compromete acesso privilegiado (ADR-0010).

## O que muda

- MFA obrigatório para toda identidade com privilégio; sem fator registrado, o login leva a
  **enrolamento obrigatório**, não a acesso.
- WebAuthn/passkey como fator padrão; TOTP como fallback restrito; SMS/e-mail não suportados
  para privilégio.
- Níveis de garantia **L1/L2/L3** com step-up e frescor, expressos em `acr`/`amr`.
- Política de MFA por organização, com a mais restritiva vencendo no tenant ativo.
- Códigos de recuperação e processo de recuperação auditado que **não** vira backdoor.
- Anti-abuso: limitação de taxa, bloqueio progressivo, detecção de credential stuffing.

## Impacto

- **Depende de:** 001, 002, 003.
- **Bloqueia:** 004 (break-glass), 006 (claims `acr`/`amr`), 008 (step-up no console).
- **Risco:** WebAuthn exige HTTPS e RP ID estável — impacta topologia e clientes com
  infraestrutura legada.
