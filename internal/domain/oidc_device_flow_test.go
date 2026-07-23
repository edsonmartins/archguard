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

// Um token de device flow não autoriza operação L3 (cenário "Device flow em
// operação crítica").
func TestDeviceFlowBlocksL3(t *testing.T) {
	// Device flow + L3 -> negado.
	if err := DeviceFlowAuthorize(L3, true); !errors.Is(err, ErrL3ViaDeviceFlow) {
		t.Fatalf("L3 via device flow deveria ser negado: %v", err)
	}
	// Device flow + L1/L2 -> ok.
	for _, lvl := range []AssuranceLevel{L1, L2} {
		if err := DeviceFlowAuthorize(lvl, true); err != nil {
			t.Fatalf("device flow deveria permitir %s: %v", lvl, err)
		}
	}
	// Não device flow + L3 -> ok (o bloqueio é só do device flow).
	if err := DeviceFlowAuthorize(L3, false); err != nil {
		t.Fatalf("L3 fora do device flow não deveria ser bloqueado por esta regra: %v", err)
	}
}
