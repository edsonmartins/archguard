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

// Configuração do Cypress para o E2E do ArchGuard (pacote 008, T-020) — separada da do
// upstream (cypress.config.js, specs legados do Casdoor). Roda só os specs de cypress/e2e/
// archguard/**. A baseUrl aponta para a stack sob teste (backend servindo o console +
// /api/v1); default :8000 (imagem do fork), sobreponível por ARCHGUARD_E2E_URL.
const {defineConfig} = require("cypress");

module.exports = defineConfig({
  e2e: {
    baseUrl: process.env.ARCHGUARD_E2E_URL || "http://localhost:8000",
    specPattern: "cypress/e2e/archguard/**/*.cy.js",
    supportFile: "cypress/support/e2e.js",
    video: false,
    retries: {runMode: 2, openMode: 0},
  },
});
