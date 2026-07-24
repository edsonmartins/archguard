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
	"strings"
	"testing"
)

func TestSensitiveKey(t *testing.T) {
	for _, k := range []string{"password", "Client-Secret", "clientSecret", "client_secret",
		"Authorization", "access_token", "refresh_token", "primary_email", "e-mail", "TOTP", "seed"} {
		if !SensitiveKey(k) {
			t.Fatalf("chave %q deveria ser sensível", k)
		}
	}
	for _, k := range []string{"organization_id", "trace_id", "membership_id", "status", "subject", "credential_ref"} {
		if SensitiveKey(k) {
			t.Fatalf("chave %q NÃO deveria ser sensível", k)
		}
	}
}

// spec "Token em log": um token/credencial em texto livre é redigido antes da
// emissão.
func TestRedactValueScrubsSecrets(t *testing.T) {
	jwt := "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiIxMjMifQ.abc-DEF_123"
	cases := map[string]string{
		"token=" + jwt:                             "[REDACTED-JWT]",
		"Authorization: Bearer " + jwt:             "[REDACTED-AUTH]",
		"user ana.souza+ops@empresa.com logou":     "[REDACTED-EMAIL]",
		"nada sensível aqui, org=123 status=ativo": "", // passa sem redação
	}
	for in, mustContain := range cases {
		out := RedactValue(in)
		if mustContain != "" && !strings.Contains(out, mustContain) {
			t.Fatalf("RedactValue(%q)=%q deveria conter %q", in, out, mustContain)
		}
		if ContainsSensitive(out) {
			t.Fatalf("saída ainda contém dado sensível: %q", out)
		}
	}
}

// spec "Identificação de usuário": um e-mail nunca sai; um pseudônimo estável passa.
func TestRedactAttr(t *testing.T) {
	// Chave sensível -> valor inteiro redigido.
	if RedactAttr("password", "qualquer-coisa") != Redacted {
		t.Fatalf("valor de chave sensível deveria ser redigido")
	}
	if RedactAttr("primary_email", "ana@cli.com") != Redacted {
		t.Fatalf("e-mail por chave deveria ser redigido")
	}
	// Chave não sensível -> valor escrutinado (e-mail embutido some).
	if got := RedactAttr("detail", "login de ana@cli.com"); ContainsSensitive(got) {
		t.Fatalf("e-mail embutido deveria ser redigido: %q", got)
	}
	// Pseudônimo estável passa (subject opaco não é e-mail).
	if got := RedactAttr("subject", "aB3xK_9Qz-Lm"); got != "aB3xK_9Qz-Lm" {
		t.Fatalf("pseudônimo estável deveria passar, veio %q", got)
	}
}
