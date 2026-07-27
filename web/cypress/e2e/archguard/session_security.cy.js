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

// Sessão e fail-closed do plano de controle (pacote 008, T-020 Fase A). Verifica que o
// /api/v1 EXIGE sessão de verdade (não é a UI que esconde — a API nega), o oposto do
// anti-padrão "autorização no frontend"; e que o contexto de sessão resolve com o shape
// esperado após o login. Não exige step-up (leituras L1).
describe("ArchGuard — sessão e fail-closed (L1)", () => {
  it("nega o /api/v1 sem sessão — 401 (a API é o controle, não a UI)", () => {
    // Sem login (testIsolation limpa os cookies antes de cada teste): uma chamada direta
    // ao plano de controle deve ser recusada.
    cy.request({url: "/api/v1/grants", failOnStatusCode: false})
      .its("status").should("eq", 401);
    cy.request({url: "/api/v1/audit/timeline", failOnStatusCode: false})
      .its("status").should("eq", 401);
  });

  it("resolve o contexto de sessão com o shape esperado após o login", () => {
    cy.cpLogin();
    cy.request("/api/v1/session").then((res) => {
      expect(res.status).to.eq(200);
      expect(res.body).to.have.property("identity_id").and.to.be.a("string").and.not.be.empty;
      expect(res.body).to.have.property("proven_aal");
      // O admin é membro de um tenant → a org ativa está presente (base do selo de tenant).
      expect(res.body).to.have.property("organization_id");
    });
  });

  it("lista os tenants do usuário (base do seletor de tenant)", () => {
    cy.cpLogin();
    cy.request("/api/v1/tenants").then((res) => {
      expect(res.status).to.eq(200);
      expect(res.body).to.have.property("tenants").and.to.be.an("array");
      // Ao menos um tenant ativo (o admin built-in tem membership).
      expect(res.body.tenants.length).to.be.greaterThan(0);
      expect(res.body.tenants.some((t) => t.active)).to.eq(true);
    });
  });
});
