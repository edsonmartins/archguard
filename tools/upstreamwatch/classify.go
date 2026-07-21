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

// Command upstreamwatch triages new upstream commits for cherry-pick (ADR-0003).
// It lists commits on vendor/upstream since the point recorded in LAST_SYNC.md
// and classifies each by nature and touched paths, producing a triage queue.
package main

import (
	"regexp"
	"strings"
)

// Class is a triage classification (ADR-0003 §"Triagem por classe").
type Class string

const (
	// Security: CVE / auth / crypto fix. SLA 72h. Cherry-pick or documented
	// mitigation.
	Security Class = "SECURITY"
	// DivergentReview: touches a subsystem ArchGuard diverged from
	// (DIVERGENCE.md) — never apply automatically, requires manual review.
	DivergentReview Class = "DIVERGENT-REVIEW"
	// OutOfScope: touches only removed scope (AI/MCP, payment, non-curated
	// providers, non-PostgreSQL) — discard.
	OutOfScope Class = "OUT-OF-SCOPE"
	// Refactor: broad upstream refactor — discard (only in a major rebase).
	Refactor Class = "REFACTOR"
	// FeaturePAM: a feature possibly aligned to PAM — needs an ADR/RFC before
	// importing.
	FeaturePAM Class = "FEATURE-PAM"
	// BugfixCandidate: a fix in a non-divergent subsystem — cherry-pick if it
	// applies cleanly.
	BugfixCandidate Class = "BUGFIX-CANDIDATE"
)

var (
	securityRe = regexp.MustCompile(`(?i)(\bcve-\d|vulnerab|\bsecurity\b|auth\w*\s+(fix|bypass)|\bbypass\b|injection|\bxss\b|\bcsrf\b|\bssrf\b|\brce\b|privilege\s+escal|\bcrypto\b|signature\s+forg|jwt\b.*\b(forg|bypass))`)
	refactorRe = regexp.MustCompile(`(?i)\b(refactor|rewrite|cleanup|restructure|reorganize)\b`)
	featureRe  = regexp.MustCompile(`(?i)^(feat|feature)\b|\badds?\b\s+support`)
)

// divergentPrefixes are path prefixes where ArchGuard diverged structurally from
// upstream (mirrors DIVERGENCE.md). A commit touching one needs manual review —
// a cherry-pick there is likely to conflict or reintroduce removed behavior.
var divergentPrefixes = []string{
	"object/ormer.go", "object/check.go", "object/organization.go",
	"object/token_jwt.go", "object/provider.go", "object/user.go",
	"object/init_data", "object/syncer_database.go", "conf/app.conf",
	"main.go", "routers/",
	// Pacote 002 (identity-multitenancy): o modelo de identidade/membership/
	// sessão do ArchGuard convive com estas superfícies legadas — commit do
	// upstream que mude a semântica delas exige revisão manual (DIVERGENCE.md).
	"object/role.go", "object/session.go", "object/invitation",
}

// outOfScopePrefixes are paths for removed features (ADR-0015/ADR-0019). A commit
// touching ONLY these is discarded.
var outOfScopePrefixes = []string{
	"mcp/", "mcpself/", "pp/", "sync/", "sync_v2/", "ldap/", "faceId/", "idv/",
	"object/agent", "object/server", "object/openclaw", "object/payment",
	"object/order", "object/product", "object/plan", "object/pricing",
	"object/subscription", "log/agent_openclaw",
	"idp/wechat", "idp/qq", "idp/baidu", "idp/alipay", "idp/dingtalk",
	"idp/weibo", "idp/gitee", "idp/lark", "idp/wecom", "idp/infoflow",
	"idp/douyin", "idp/kwai", "idp/bilibili", "idp/metamask", "idp/web3",
	"idp/telegram", "idp/twitter", "idp/facebook", "idp/linkedin", "idp/goth",
	"notification/matrix", "notification/telegram", "notification/discord",
	"captcha/aliyun",
}

func anyPrefix(path string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// Classify assigns a triage class to a commit given its message and touched
// files. Order matters: security first (highest urgency), then divergence
// (manual review), then out-of-scope, then nature. Frontend (web/) and pure
// docs are treated as out of scope for the backend fork.
func Classify(message string, files []string) (Class, string) {
	if securityRe.MatchString(message) {
		return Security, "correção de segurança (mensagem) — SLA 72h"
	}

	touchesDivergent := false
	allOutOfScope := len(files) > 0
	for _, f := range files {
		if anyPrefix(f, divergentPrefixes) {
			touchesDivergent = true
		}
		if !anyPrefix(f, outOfScopePrefixes) && !strings.HasPrefix(f, "web/") {
			allOutOfScope = false
		}
	}

	if touchesDivergent {
		return DivergentReview, "toca subsistema divergente (DIVERGENCE.md) — revisão manual"
	}
	if allOutOfScope {
		return OutOfScope, "toca apenas escopo removido — descartar"
	}
	if refactorRe.MatchString(message) {
		return Refactor, "refactor amplo — descartar (só em rebase de major)"
	}
	if featureRe.MatchString(message) {
		return FeaturePAM, "feature — exige ADR/RFC antes de importar"
	}
	return BugfixCandidate, "correção em subsistema não divergente — avaliar cherry-pick"
}
