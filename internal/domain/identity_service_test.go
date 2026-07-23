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

package domain

import (
	"errors"
	"testing"
)

// Conta de serviço não autentica por fluxo interativo (cenário "Login
// interativo de conta de serviço").
func TestServiceAccountNoInteractiveLogin(t *testing.T) {
	human, err := NewIdentity(IdentityHuman)
	if err != nil {
		t.Fatalf("NewIdentity human: %v", err)
	}
	if !human.Type.AllowsInteractiveLogin() {
		t.Fatalf("humano deveria poder login interativo")
	}
	if err := human.EnsureInteractiveLoginAllowed(); err != nil {
		t.Fatalf("humano não deveria ser barrado: %v", err)
	}

	svc, err := NewIdentity(IdentityService)
	if err != nil {
		t.Fatalf("NewIdentity service: %v", err)
	}
	if svc.Type.AllowsInteractiveLogin() {
		t.Fatalf("conta de serviço NÃO deveria permitir login interativo")
	}
	if err := svc.EnsureInteractiveLoginAllowed(); !errors.Is(err, ErrInteractiveLoginForbidden) {
		t.Fatalf("conta de serviço: err = %v, quero ErrInteractiveLoginForbidden", err)
	}
}
