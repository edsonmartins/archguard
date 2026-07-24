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

import "strings"

// Legacy edge channels (pacote 009, RFC-0007 §6 / ADR-0019). ArchGuard keeps ONE
// legacy edge protocol for equipment that cannot speak OIDC: the embedded RADIUS
// server. The embedded LDAP SERVER was REMOVED (ADR-0019: GPL-2.0 goldap +
// ADR-0015). The client-side LDAP/AD connector (pacote 009 T-002) is NOT a legacy
// channel — it is the sanctioned directory integration and stays.
//
// Normative restrictions on a legacy channel (I-4.4 / RFC-0007 §6):
//   - DISABLED BY DEFAULT — it is never on by accident.
//   - Minimal scope, audited, and flagged as legacy.
//   - NEVER a path to a privileged (L3) operation — it carries no acr nor
//     correlation. That L3 block is LegacyChannelSession (T-015).

// LegacyChannel identifies a legacy edge channel.
type LegacyChannel string

const (
	// LegacyRADIUS is the embedded RADIUS server — the only legacy edge channel.
	LegacyRADIUS LegacyChannel = "radius"
)

// LegacyChannelConfig is the deployment's stance on legacy channels. The zero
// value has every channel DISABLED, which is the safe default.
type LegacyChannelConfig struct {
	RADIUSEnabled bool
}

// NewLegacyChannelConfig parses the RADIUS enable flag CONSERVATIVELY: only an
// explicit affirmative ("true", "1", "yes", "on") enables the embedded RADIUS
// server; anything else — absent, empty, "false", or malformed — leaves it
// DISABLED. A legacy channel never comes up by accident (I-4.4).
func NewLegacyChannelConfig(radiusEnableFlag string) LegacyChannelConfig {
	return LegacyChannelConfig{RADIUSEnabled: parseAffirmative(radiusEnableFlag)}
}

// Enabled reports whether the given legacy channel is enabled. An unknown channel
// is never enabled.
func (c LegacyChannelConfig) Enabled(ch LegacyChannel) bool {
	switch ch {
	case LegacyRADIUS:
		return c.RADIUSEnabled
	default:
		return false
	}
}

// parseAffirmative reports whether a config value is an explicit "yes". The default
// (and every ambiguous value) is false — the conservative reading for a security
// toggle that opens a legacy channel.
func parseAffirmative(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true", "1", "yes", "on", "enabled":
		return true
	default:
		return false
	}
}
