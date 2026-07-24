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

// Degraded signing mode (pacote 010, T-015 / design 010 §"Modo degradado"). The
// signer keeps a SHORT cache of its signing capability so a momentary vault blip
// does not halt ordinary token issuance. But an L3 operation NEVER rides the
// degraded path — it requires a healthy vault, and fails closed when the vault is
// unavailable. Once the cache expires, all signing degrades to denial (fail-closed).

// SigningAvailability decides whether a signing operation may proceed, given the
// vault's health, whether the short capability cache is still fresh, and the
// operation's assurance level. It returns whether the operation is allowed and,
// when allowed, whether it is running DEGRADED (on the cache, not a live vault).
//
//   - Vault healthy        ⇒ (allowed, not degraded), any level.
//   - Vault down, L3        ⇒ (DENIED): L3 never degrades — fail-closed.
//   - Vault down, <L3, cache fresh ⇒ (allowed, DEGRADED): ordinary issuance
//     continues briefly.
//   - Vault down, cache expired ⇒ (DENIED): the blip outlasted the cache.
func SigningAvailability(vaultHealthy, cacheFresh bool, level AssuranceLevel) (allowed, degraded bool) {
	if vaultHealthy {
		return true, false
	}
	if level == L3 {
		return false, false // L3 requires the vault; never degraded
	}
	if cacheFresh {
		return true, true
	}
	return false, false
}
