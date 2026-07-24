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

package samlfed

import (
	"errors"
	"testing"

	"github.com/casdoor/casdoor/internal/domain"
	saml2 "github.com/russellhaering/gosaml2"
	"github.com/russellhaering/gosaml2/types"
)

// fakeSP stands in for *saml2.SAMLServiceProvider: it returns a preset
// AssertionInfo/error, so the mapping is tested without a signed fixture.
type fakeSP struct {
	info *saml2.AssertionInfo
	err  error
}

func (f *fakeSP) RetrieveAssertionInfo(string) (*saml2.AssertionInfo, error) {
	return f.info, f.err
}

func attr(value string) types.Attribute {
	return types.Attribute{Values: []types.AttributeValue{{Value: value}}}
}

func newValidator(sp AssertionValidator) *Validator {
	return NewValidator(sp, "entra", "email", "displayName", "acr")
}

// Assertion válida é mapeada; o acr do IdP é capturado como INFORMATIVO e nunca
// autoriza L3.
func TestValidateResponseMapsAndNeverL3(t *testing.T) {
	info := &saml2.AssertionInfo{
		NameID: "ana@cli.com",
		Values: saml2.Values{
			"email":       attr("ana@cli.com"),
			"displayName": attr("Ana Souza"),
			"acr":         attr("urn:acr:strong"),
		},
	}
	fed, err := newValidator(&fakeSP{info: info}).ValidateResponse("<encoded>")
	if err != nil {
		t.Fatalf("ValidateResponse: %v", err)
	}
	if fed.Email != "ana@cli.com" || fed.DisplayName != "Ana Souza" || fed.Provider != "entra" {
		t.Fatalf("mapeamento inesperado: %+v", fed)
	}
	if fed.IdPACR != "urn:acr:strong" {
		t.Fatalf("acr do IdP deveria ser capturado como informativo")
	}
	if fed.AuthorizesL3() {
		t.Fatalf("acr de terceiro JAMAIS autoriza L3")
	}
}

// Assinatura inválida (erro do SP) propaga como erro — nunca uma identidade.
func TestValidateResponseSignatureError(t *testing.T) {
	_, err := newValidator(&fakeSP{err: errors.New("assinatura inválida")}).ValidateResponse("<x>")
	if err == nil {
		t.Fatalf("assinatura inválida deveria falhar")
	}
}

// Condições não satisfeitas (fora da janela / audiência errada) => rejeição
// fail-closed, não um warning ignorado.
func TestValidateResponseRejectsBadConditions(t *testing.T) {
	for _, w := range []*saml2.WarningInfo{
		{InvalidTime: true},
		{NotInAudience: true},
	} {
		info := &saml2.AssertionInfo{NameID: "ana@cli.com", WarningInfo: w,
			Values: saml2.Values{"email": attr("ana@cli.com")}}
		_, err := newValidator(&fakeSP{info: info}).ValidateResponse("<x>")
		if !errors.Is(err, ErrAssertionConditions) {
			t.Fatalf("condição %+v deveria ser rejeitada, veio %v", w, err)
		}
	}
}

// Sem e-mail (nem atributo nem NameID em formato de e-mail) => rejeitado.
func TestValidateResponseRequiresEmail(t *testing.T) {
	info := &saml2.AssertionInfo{NameID: "S-1-5-21-nao-e-email", Values: saml2.Values{}}
	_, err := newValidator(&fakeSP{info: info}).ValidateResponse("<x>")
	if !errors.Is(err, domain.ErrFederatedEmailRequired) {
		t.Fatalf("sem e-mail deveria ser ErrFederatedEmailRequired, veio %v", err)
	}
}

// NameID em formato de e-mail é aceito como e-mail quando não há atributo.
func TestValidateResponseNameIDEmailFallback(t *testing.T) {
	info := &saml2.AssertionInfo{NameID: "bob@cli.com", Values: saml2.Values{}}
	fed, err := newValidator(&fakeSP{info: info}).ValidateResponse("<x>")
	if err != nil || fed.Email != "bob@cli.com" {
		t.Fatalf("NameID e-mail deveria virar e-mail: %+v err=%v", fed, err)
	}
}
