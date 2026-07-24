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

// I-3.2 / INV-7 na TELEMETRIA (pacote 010, T-005 / spec "Higiene de dados
// sensíveis em telemetria" — "Verificação automatizada: se a suíte detecta dado
// sensível, o build é rejeitado"). Este teste é o guardião de completude do
// redator: alimenta um corpus abrangente de segredos/tokens/PII e QUEBRA O BUILD
// se qualquer amostra sobreviver à redação. Enfraquecer o redator falha aqui.

import (
	"strings"
	"testing"

	"github.com/casdoor/casdoor/internal/domain"
)

const sampleJWT = "eyJhbGciOiJSUzI1NiIsImtpZCI6ImsxIn0.eyJzdWIiOiJhYmMiLCJvcmciOiJvMSJ9.sig-Nature_123-xyz"

// Nenhum valor de texto livre com segredo/token/PII sobrevive à redação.
func TestINV7TelemetryFreeTextRedacted(t *testing.T) {
	samples := []string{
		"token=" + sampleJWT,
		"id_token=" + sampleJWT + " emitido",
		"Authorization: Bearer " + sampleJWT,
		"authorization: Basic dXNlcjpwYXNz",
		"contato ana.souza+ops@empresa.com.br falhou",
		"emails: a@b.com, c@d.org no lote",
	}
	for _, s := range samples {
		out := domain.RedactValue(s)
		if domain.ContainsSensitive(out) {
			t.Fatalf("BUILD REJEITADO: dado sensível sobreviveu à redação em %q -> %q", s, out)
		}
	}
}

// Toda chave sensível redige o valor inteiro, independentemente do conteúdo.
func TestINV7TelemetrySensitiveKeysRedacted(t *testing.T) {
	sensitive := []string{
		"password", "passwd", "secret", "client_secret", "clientSecret",
		"token", "access_token", "refresh_token", "id_token",
		"authorization", "Authorization", "cookie", "Set-Cookie",
		"api_key", "apiKey", "private_key", "credential", "bearer", "jwt",
		"seed", "totp", "otp", "email", "e-mail", "primary_email", "mail",
	}
	for _, k := range sensitive {
		if !domain.SensitiveKey(k) {
			t.Fatalf("BUILD REJEITADO: chave sensível %q não é reconhecida — telemetria vazaria seu valor", k)
		}
		if got := domain.RedactAttr(k, "valor-secreto-qualquer"); got != domain.Redacted {
			t.Fatalf("BUILD REJEITADO: valor de %q não foi redigido: %q", k, got)
		}
	}
}

// Pseudônimos e identificadores operacionais NÃO são redigidos (o sinal continua
// útil): a telemetria referencia o usuário por pseudônimo estável, nunca e-mail.
func TestINV7TelemetryPseudonymsPass(t *testing.T) {
	operational := map[string]string{
		"subject":         "aB3xK_9Qz-Lm",
		"organization_id": "018f2b7c-1234-7890-abcd-ef0123456789",
		"trace_id":        "4bf92f3577b34da6a3ce929d0e0e4736",
		"membership_id":   "018f2b7c-9999-7890-abcd-ef0123456789",
		"credential_ref":  "vault://kv/data/org/ad-bind",
	}
	for k, v := range operational {
		if got := domain.RedactAttr(k, v); strings.Contains(got, "REDACTED") {
			t.Fatalf("identificador operacional %q=%q não deveria ser redigido, veio %q", k, v, got)
		}
	}
}
