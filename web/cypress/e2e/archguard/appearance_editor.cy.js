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

// Smoke E2E do editor de aparência (pacote 008, T-022). Tela NOSSA e aditiva sobre os
// campos de personalização do app. Login como admin (perfil dev) e abre a aparência do
// app-built-in: valida que a prévia + o painel de propriedades renderizam e que o atalho
// "Edição avançada" leva à tela herdada.
describe("ArchGuard — editor de aparência (T-022)", () => {
  beforeEach(() => {
    cy.cpLogin();
  });

  it("renderiza a tela de aparência do app-built-in (preview + painel)", () => {
    cy.intercept("GET", "**/api/get-application*").as("getApp");
    cy.visit("/appearance/admin/app-built-in");
    cy.wait("@getApp").its("response.statusCode").should("eq", 200);
    // Cabeçalho da tela nova.
    cy.contains(/Appearance|Aparência/i).should("exist");
    // Painel de propriedades (campo de logo) e ações.
    cy.contains(/Logo URL|URL do logo/i).should("exist");
    cy.contains("button", /Save|Salvar/i).should("exist");
    cy.contains("button", /Advanced edit|Edição avançada/i).should("exist");
  });

  it("'Edição avançada' navega para a tela herdada da aplicação", () => {
    cy.visit("/appearance/admin/app-built-in");
    cy.contains("button", /Advanced edit|Edição avançada/i).click();
    cy.location("pathname").should("include", "/applications/admin/app-built-in");
  });
});
