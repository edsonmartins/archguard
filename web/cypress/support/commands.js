// ***********************************************
// This example commands.js shows you how to
// create various custom commands and overwrite
// existing commands.
//
// For more comprehensive examples of custom
// commands please read more here:
// https://on.cypress.io/custom-commands
// ***********************************************
//
//
// -- This is a parent command --
// Cypress.Commands.add('login', (email, password) => { ... })
//
//
// -- This is a child command --
// Cypress.Commands.add('drag', { prevSubject: 'element'}, (subject, options) => { ... })
//
//
// -- This is a dual command --
// Cypress.Commands.add('dismiss', { prevSubject: 'optional'}, (subject, options) => { ... })
//
//
// -- This will overwrite an existing command --
// Cypress.Commands.overwrite('visit', (originalFn, url, options) => { ... })
const selector = {
  username: "#input",
  password: "#normal_login_password",
  loginButton: ".ant-btn",
};
Cypress.Commands.add('login', ()=>{
  cy.visit("http://localhost:7001", {
    onBeforeLoad(win) {
      // Disable the page tour so its popover never covers elements the tests click
      win.localStorage.setItem("isTourVisible", "false");
    },
  });
  cy.get(selector.username).type("admin");
  cy.get(selector.password).type("123");
  cy.get(selector.loginButton).click();
  cy.url().should("eq", "http://localhost:7001/");
})

// cpLogin: login de API contra a baseUrl (ArchGuard E2E). O POST /api/login estabelece a
// sessão do Casdoor E dispara a ponte que cria a auth_session do plano de controle — o
// cookie resultante resolve o /api/v1. Credenciais dev built-in (admin/123).
Cypress.Commands.add('cpLogin', (username = 'admin', password = '123') => {
  cy.request({
    method: 'POST',
    url: '/api/login',
    body: {
      application: 'app-built-in',
      organization: 'built-in',
      username,
      password,
      // Explícito: sem isso o Casdoor pode cair no path de telefone ("Phone number is
      // invalid in your region") em apps cuja config de signin não priorize senha.
      signinMethod: 'Password',
      autoSignin: true,
      type: 'login',
    },
  }).its('body.status').should('eq', 'ok');
})
