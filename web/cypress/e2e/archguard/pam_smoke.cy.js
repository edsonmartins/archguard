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

// Smoke E2E dos fluxos L1 do console PAM (pacote 008, T-020 Fase A). NÃO exige step-up:
// login como admin (perfil dev) e navega as telas novas, afirmando que renderizam e que
// o /api/v1 responde 200 — valida ponta-a-ponta a fiação console↔/api/v1 que construímos.
// Os fluxos L3 (revogar, break-glass, verificar cadeia) exigem WebAuthn e são a Fase B.
describe("ArchGuard — console PAM smoke (L1)", () => {
  beforeEach(() => {
    // Sessão do plano de controle via login de API (a ponte de login estabelece a
    // auth_session; o cookie carrega o vínculo para o /api/v1).
    cy.cpLogin();
  });

  it("resolve a sessão do plano de controle (/api/v1/session)", () => {
    cy.request("/api/v1/session").its("status").should("eq", 200);
  });

  it("Acesso Privilegiado → Concessões vigentes renderiza e chama /api/v1/grants", () => {
    cy.intercept("GET", "**/api/v1/grants").as("grants");
    cy.visit("/grants");
    cy.wait("@grants").its("response.statusCode").should("eq", 200);
    cy.contains(/Active grants|Concessões|No active grants|Nenhuma/i).should("exist");
  });

  it("Auditoria → timeline + indicador de integridade sempre visível", () => {
    cy.intercept("GET", "**/api/v1/audit/timeline*").as("timeline");
    cy.visit("/audit");
    cy.wait("@timeline").its("response.statusCode").should("eq", 200);
    // O banner de integridade da cadeia é SEMPRE visível (agregado honesto).
    cy.contains(/not verified this session|não verificada|intact|íntegra|DIVERGENCE|DIVERGÊNCIA/i).should("exist");
  });

  it("Break-glass → fila de aprovação renderiza e chama /api/v1/breakglass/pending", () => {
    cy.intercept("GET", "**/api/v1/breakglass/pending").as("pending");
    cy.visit("/breakglass/queue");
    cy.wait("@pending").its("response.statusCode").should("eq", 200);
    cy.contains(/approval|aprovação|No break-glass|Nenhuma solicitação/i).should("exist");
  });

  it("Break-glass → formulário de solicitação renderiza os campos obrigatórios", () => {
    cy.visit("/breakglass/request");
    cy.contains(/Request break-glass|Solicitar break-glass/i).should("exist");
    cy.contains(/Justification|Justificativa/i).should("exist");
    cy.contains(/Incident|Incidente/i).should("exist");
  });
});
