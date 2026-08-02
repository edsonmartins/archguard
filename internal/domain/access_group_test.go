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

	"github.com/google/uuid"
)

// TestNewAccessGroup: nome obrigatório; displayName default = name; Ref qualificado.
func TestNewAccessGroup(t *testing.T) {
	org := uuid.New()
	g, err := NewAccessGroup(org, "dba", "")
	if err != nil {
		t.Fatalf("NewAccessGroup: %v", err)
	}
	if g.DisplayName != "dba" {
		t.Errorf("displayName default deveria ser o name, veio %q", g.DisplayName)
	}
	if g.Ref() != Qualify(org, TypeGroup, g.ID.String()) {
		t.Errorf("ref = %q", g.Ref())
	}
	if _, err := NewAccessGroup(org, "", ""); !errors.Is(err, ErrInvalidAccessGroup) {
		t.Error("nome vazio deveria falhar")
	}
	if _, err := NewAccessGroup(uuid.Nil, "x", ""); !errors.Is(err, ErrInvalidAccessGroup) {
		t.Error("org nula deveria falhar")
	}
}
