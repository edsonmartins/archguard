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

package invariants

// INV-7 / I-3.2 nos CLAIMS DO TOKEN (ROADMAP-endurecimento 0.4). O access token é legível por
// quem o detém — navegador, proxy, log, toda relying party —, então credencial ou segredo de MFA
// nele anula o fator que protege. O formato legado "JWT" serializava o usuário inteiro (incluindo
// totpSecret, recoveryCodes, passwordSalt e a senha das contas gerenciadas), e o "JWT-Custom"
// copia qualquer campo por reflexão.
//
// Este guardião quebra o build se:
//   - um campo do User com nome de segredo não estiver na lista de proibidos (completude: campo
//     novo herdado do upstream entra bloqueado ou com justificativa explícita aqui);
//   - um campo proibido for emitido por alguma estrutura de claims (tag json diferente de "-").

import (
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/casdoor/casdoor/object"
)

// Nome com cara de credencial, segredo ou dado pessoal de segurança.
var secretLikeField = regexp.MustCompile(`(?i)password|salt|secret|token|recovery|credential|` +
	`managedaccount|mfaaccount|invitation|idcard|ip$|signinwrong`)

// Campos que casam com o padrão mas NÃO são segredo. Cada entrada exige justificativa; ampliar
// esta lista é decisão de revisão, não atalho para o teste passar.
var secretLikeButAllowed = map[string]string{
	"PasswordType":           "algoritmo do hash (ex.: bcrypt), não o hash",
	"IdCardType":             "tipo do documento, sem o número",
	"Invitation":             "nome do convite, não o código",
	"LastChangePasswordTime": "data da última troca, sem a senha",
	"NeedUpdatePassword":     "flag booleana",
}

func TestINV7TokenClaimsForbiddenListIsComplete(t *testing.T) {
	userType := reflect.TypeOf(object.User{})
	for i := 0; i < userType.NumField(); i++ {
		name := userType.Field(i).Name
		if !secretLikeField.MatchString(name) {
			continue
		}
		if _, ok := secretLikeButAllowed[name]; ok {
			continue
		}
		if !object.IsForbiddenTokenField(name) {
			t.Errorf("User.%s parece segredo/dado pessoal e não está em forbiddenTokenFields "+
				"(object/token_claims_guard.go) nem justificado em secretLikeButAllowed", name)
		}
	}
}

func TestINV7TokenClaimStructsDoNotEmitForbiddenFields(t *testing.T) {
	for _, s := range []interface{}{object.UserWithoutThirdIdp{}, object.UserStandard{}, object.UserShort{}} {
		typ := reflect.TypeOf(s)
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if !object.IsForbiddenTokenField(field.Name) {
				continue
			}
			tag := strings.Split(field.Tag.Get("json"), ",")[0]
			if tag != "-" {
				t.Errorf("%s.%s é proibido em token e é emitido como claim %q — marque json:\"-\"",
					typ.Name(), field.Name, tag)
			}
		}
	}
}

func TestINV7ApplicationCannotRequestForbiddenTokenFields(t *testing.T) {
	app := &object.Application{
		TokenFields:     []string{"TotpSecret", "Properties.oauth_GitHub_accessToken"},
		TokenAttributes: []*object.JwtItem{{Name: "x", Category: "Existing Field", Value: "RecoveryCodes"}},
	}
	got := object.ForbiddenTokenFieldsOf(app)
	if len(got) != 3 {
		t.Fatalf("configuração com 3 campos proibidos detectou %v", got)
	}
}
