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
	"strings"
	"testing"

	"github.com/google/uuid"
)

// Um conjunto recém-gerado: N códigos, cada credencial na forma INV-7 (só
// verifier de uso único, sem segredo reversível), AAL2, e o texto plano nunca
// aparece na credencial.
func TestGenerateRecoveryCodesShape(t *testing.T) {
	id := uuid.New()
	plain, creds, err := GenerateRecoveryCodes(id, 10)
	if err != nil {
		t.Fatalf("GenerateRecoveryCodes: %v", err)
	}
	if len(plain) != 10 || len(creds) != 10 {
		t.Fatalf("esperava 10 códigos, veio %d/%d", len(plain), len(creds))
	}
	seen := map[string]bool{}
	for i, c := range creds {
		if c.Type != FactorRecoveryCode || !c.WellFormed() {
			t.Fatalf("credencial de recuperação malformada: %+v", c)
		}
		if c.AAL != AAL2 {
			t.Fatalf("recovery deveria ser AAL2, veio %s", c.AAL)
		}
		if len(c.Verifier) == 0 || c.SecretRef != "" || len(c.PublicMaterial) != 0 {
			t.Fatalf("forma INV-7 violada: %+v", c)
		}
		// O texto plano não pode estar na credencial (nem como verifier bruto).
		if string(c.Verifier) == plain[i] {
			t.Fatalf("verifier não deveria ser o código em texto plano")
		}
		if seen[plain[i]] {
			t.Fatalf("código duplicado gerado: %s", plain[i])
		}
		seen[plain[i]] = true
	}
}

// Um código válido casa com sua credencial; a verificação é robusta a
// formatação (maiúsculas, espaços em vez de hífen).
func TestMatchRecoveryCode(t *testing.T) {
	id := uuid.New()
	plain, creds, err := GenerateRecoveryCodes(id, 5)
	if err != nil {
		t.Fatalf("GenerateRecoveryCodes: %v", err)
	}

	matched, err := MatchRecoveryCode(creds, plain[2])
	if err != nil {
		t.Fatalf("código válido deveria casar: %v", err)
	}
	if matched != creds[2].ID {
		t.Fatalf("casou credencial errada")
	}

	// Robustez de transcrição: mesmo código, maiúsculas e espaços no lugar de hífen.
	reformatted := strings.ToUpper(strings.ReplaceAll(plain[2], "-", " "))
	if _, err := MatchRecoveryCode(creds, reformatted); err != nil {
		t.Fatalf("código reformatado deveria casar: %v", err)
	}
}

// Código desconhecido é negação (ErrNoRecoveryCode), não erro de sistema.
func TestMatchRecoveryCodeNoMatch(t *testing.T) {
	_, creds, _ := GenerateRecoveryCodes(uuid.New(), 3)
	_, err := MatchRecoveryCode(creds, "zzzz-zzzz-zzzz")
	if !errors.Is(err, ErrNoRecoveryCode) {
		t.Fatalf("código inválido: err = %v, quero ErrNoRecoveryCode", err)
	}
}

// Uso único: consumir a credencial casada (removê-la do conjunto) faz o mesmo
// código deixar de casar — não pode ser reusado.
func TestRecoveryCodeSingleUse(t *testing.T) {
	plain, creds, _ := GenerateRecoveryCodes(uuid.New(), 4)
	matched, err := MatchRecoveryCode(creds, plain[1])
	if err != nil {
		t.Fatalf("primeira apresentação deveria casar: %v", err)
	}

	// Consome: remove a credencial casada (o que o repositório faz após o uso).
	remaining := make([]Credential, 0, len(creds)-1)
	for _, c := range creds {
		if c.ID != matched {
			remaining = append(remaining, c)
		}
	}
	if _, err := MatchRecoveryCode(remaining, plain[1]); !errors.Is(err, ErrNoRecoveryCode) {
		t.Fatalf("código consumido não deveria casar de novo: %v", err)
	}
	// Os demais códigos continuam válidos.
	if _, err := MatchRecoveryCode(remaining, plain[2]); err != nil {
		t.Fatalf("outros códigos deveriam continuar válidos: %v", err)
	}
}

// Invalidação em massa: gerar um conjunto novo torna todos os códigos antigos
// inválidos contra o conjunto novo.
func TestRecoveryCodeMassInvalidation(t *testing.T) {
	id := uuid.New()
	oldPlain, _, _ := GenerateRecoveryCodes(id, 5)
	_, newCreds, _ := GenerateRecoveryCodes(id, 5)
	for i, code := range oldPlain {
		if _, err := MatchRecoveryCode(newCreds, code); !errors.Is(err, ErrNoRecoveryCode) {
			t.Fatalf("código antigo %d não deveria casar no conjunto novo", i)
		}
	}
}

// Faixa de quantidade validada; 0 usa o padrão.
func TestGenerateRecoveryCodesBounds(t *testing.T) {
	id := uuid.New()
	if _, creds, err := GenerateRecoveryCodes(id, 0); err != nil || len(creds) != defaultRecoveryCodes {
		t.Fatalf("n=0 deveria usar o padrão %d: len=%d err=%v", defaultRecoveryCodes, len(creds), err)
	}
	if _, _, err := GenerateRecoveryCodes(id, maxRecoveryCodes+1); err == nil {
		t.Fatalf("quantidade acima do máximo deveria falhar")
	}
}
