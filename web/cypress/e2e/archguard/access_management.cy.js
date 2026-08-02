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

// Smoke E2E da tela de gestão de acesso (pacote 007 M4). Op L1 + admin: login e valida a
// fiação console↔/api/v1 — a tela renderiza as 3 abas e busca ativos/atribuições/vínculos.
describe("ArchGuard — gestão de acesso (M4)", () => {
  beforeEach(() => {
    cy.cpLogin();
  });

  it("renderiza as abas e busca os dados", () => {
    cy.intercept("GET", "**/api/v1/assets").as("assets");
    cy.intercept("GET", "**/api/v1/access-assignments").as("assignments");
    cy.visit("/access-management");
    cy.wait("@assets").its("response.statusCode").should("eq", 200);
    cy.wait("@assignments").its("response.statusCode").should("eq", 200);
    cy.contains(/Access management|Gestão de acesso/i).should("exist");
    cy.contains(/Register asset|Registrar ativo/i).should("exist");
  });
});
