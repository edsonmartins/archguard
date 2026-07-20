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

package main

import "testing"

func TestClassify(t *testing.T) {
	cases := []struct {
		name  string
		msg   string
		files []string
		want  Class
	}{
		{"security wins over everything", "fix: auth bypass in login", []string{"controllers/auth.go"}, Security},
		{"cve keyword", "patch CVE-2026-1234 in token parsing", []string{"object/token.go"}, Security},
		{"divergent subsystem needs review", "fix: correct organization lookup", []string{"object/organization.go"}, DivergentReview},
		{"only out-of-scope is discarded", "feat: add new MCP tool", []string{"mcp/util.go", "mcpself/base.go"}, OutOfScope},
		{"removed provider discarded", "fix wechat login", []string{"idp/wechat.go"}, OutOfScope},
		{"broad refactor discarded", "refactor: reorganize the storage layer", []string{"storage/storage.go"}, Refactor},
		{"pam feature needs adr", "feat: add WebAuthn step-up", []string{"object/mfa_webauthn.go"}, FeaturePAM},
		{"bugfix in neutral subsystem", "fix: nil deref in group listing", []string{"object/group.go"}, BugfixCandidate},
		{"frontend-only counts as out of scope", "feat: new login page", []string{"web/src/Login.js"}, OutOfScope},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, reason := Classify(tc.msg, tc.files)
			if got != tc.want {
				t.Fatalf("Classify(%q, %v) = %s (%s), quer %s", tc.msg, tc.files, got, reason, tc.want)
			}
		})
	}
}

func TestSecurityBeatsDivergent(t *testing.T) {
	// A security fix in a divergent subsystem is still SECURITY (72h SLA);
	// the manual-review flag comes from the reason, not by downgrading urgency.
	got, _ := Classify("fix: auth bypass", []string{"object/check.go"})
	if got != Security {
		t.Fatalf("segurança deve vencer divergência: %s", got)
	}
}
