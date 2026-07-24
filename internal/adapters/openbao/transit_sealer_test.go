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

package openbao

import (
	"context"
	"testing"
)

// O selo é assinado no cofre e o key_id carrega a versão da chave (spec
// "Assinatura de selo: a operação ocorre no cofre").
func TestTransitSealerSign(t *testing.T) {
	ft := &fakeTransit{}
	srv := ft.server(t)
	t.Cleanup(srv.Close)
	sealer := NewTransitSealer(NewWithHTTP(srv.URL, "tok", srv.Client()), "transit", "audit-seal")

	sig, keyID, err := sealer.Sign(context.Background(), []byte("conteúdo do selo"))
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if len(sig) == 0 {
		t.Fatalf("assinatura vazia")
	}
	// key_id = "<keyName>:v<version>" — carrega a versão p/ verificação pós-rotação.
	if keyID != "audit-seal:v1" {
		t.Fatalf("key_id inesperado: %q", keyID)
	}
	if ft.privateKeyRequested {
		t.Fatalf("a aplicação JAMAIS deveria pedir a chave privada do selo")
	}
}

// Falha do cofre → o selo não é produzido (fail-closed).
func TestTransitSealerFailClosed(t *testing.T) {
	// Endereço inalcançável (porta 1) + cliente com timeout padrão.
	sealer := NewTransitSealer(New("http://127.0.0.1:1", "tok"), "transit", "audit-seal")
	if _, _, err := sealer.Sign(context.Background(), []byte("x")); err == nil {
		t.Fatalf("cofre inalcançável deveria falhar (fail-closed)")
	}
}
