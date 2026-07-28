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

// Smoke E2E da tela de saúde dos subsistemas (pacote 008, T-013). Op L1 (sem step-up):
// login como admin (perfil dev) e valida a fiação console↔/api/v1/health + o AGREGADO
// HONESTO. No harness dev o subsistema "deployment" é DEGRADED (perfil não-conforme,
// ADR-0017), então o selo de topo NÃO pode ser o verde "todos operacionais".
describe("ArchGuard — saúde dos subsistemas (L1)", () => {
  beforeEach(() => {
    cy.cpLogin();
  });

  it("renderiza a tela e chama /api/v1/health", () => {
    cy.intercept("GET", "**/api/v1/health").as("health");
    cy.visit("/health");
    cy.wait("@health").its("response.statusCode").should("eq", 200);
    cy.contains(/Subsystem health|Saúde dos subsistemas/i).should("exist");
    // Os subsistemas sondados pelo backend aparecem na tabela.
    cy.contains(/database/i).should("exist");
    cy.contains(/deployment/i).should("exist");
  });

  it("agregado honesto: perfil dev (deployment degraded) não mostra verde no topo", () => {
    cy.intercept("GET", "**/api/v1/health").as("health");
    cy.visit("/health");
    cy.wait("@health").its("response.statusCode").should("eq", 200);
    // O banner de topo sinaliza pendência (degradado/indisponível), nunca "todos operacionais".
    cy.contains(/needs attention|requer atenção|unavailable|indisponível/i).should("exist");
    cy.contains(/All subsystems operational|Todos os subsistemas operacionais/i).should("not.exist");
  });
});
