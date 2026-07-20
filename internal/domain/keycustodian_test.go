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

import "testing"

func TestNormalizeEmail(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"  Alice+Ops@Empresa.COM ", "alice+ops@empresa.com"},
		{"USER@EXAMPLE.ORG", "user@example.org"},
		{"already@lower.com", "already@lower.com"},
		{"\tTabbed@x.io\n", "tabbed@x.io"},
		{"", ""},
		{"   ", ""},
	}
	for _, c := range cases {
		if got := NormalizeEmail(c.in); got != c.want {
			t.Errorf("NormalizeEmail(%q) = %q, quer %q", c.in, got, c.want)
		}
	}
}

func TestNormalizeEmailDoesNotCanonicalizeAggressively(t *testing.T) {
	// Dois endereços que canonicalização agressiva (estilo Gmail) fundiria devem
	// permanecer DISTINTOS — fundir titulares é inaceitável num PAM.
	a := NormalizeEmail("Alice+Ops@Empresa.com")
	b := NormalizeEmail("Alice@Empresa.com")
	if a == b {
		t.Errorf("+tag não deveria ser removida: %q == %q", a, b)
	}
	c := NormalizeEmail("a.lice@empresa.com")
	d := NormalizeEmail("alice@empresa.com")
	if c == d {
		t.Errorf("pontos do local-part não deveriam ser removidos: %q == %q", c, d)
	}
}
