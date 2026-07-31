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

// Smoke E2E da tela de revisão de acesso (pacote 008, T-012). Op L1 (sem step-up): login
// como admin (perfil dev) e valida a fiação console↔/api/v1 — a tela renderiza e busca os
// ativos (o seletor do alvo da revisão).
describe("ArchGuard — revisão de acesso (T-012)", () => {
  beforeEach(() => {
    cy.cpLogin();
  });

  it("renderiza a tela e busca os ativos do tenant", () => {
    cy.intercept("GET", "**/api/v1/assets").as("assets");
    cy.visit("/access-review");
    cy.wait("@assets").its("response.statusCode").should("eq", 200);
    cy.contains(/Access review|Revisão de acesso/i).should("exist");
    cy.contains(/Select an asset|Selecione um ativo/i).should("exist");
  });
});
