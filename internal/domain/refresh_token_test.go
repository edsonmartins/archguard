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
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// O segredo é entregue uma vez; só o hash é guardado, e ele casa (INV-7).
func TestNewRefreshSecret(t *testing.T) {
	s1, h1, err := NewRefreshSecret()
	if err != nil {
		t.Fatalf("NewRefreshSecret: %v", err)
	}
	if s1[:3] != "rt_" {
		t.Fatalf("segredo deveria ter prefixo rt_: %q", s1)
	}
	if !bytes.Equal(h1, HashRefreshToken(s1)) {
		t.Fatalf("o hash guardado deveria casar com o segredo")
	}
	s2, _, _ := NewRefreshSecret()
	if s1 == s2 {
		t.Fatalf("segredos deveriam ser únicos")
	}
}

// Renovação normal: rotacionar um token ativo o marca rotated e devolve um
// sucessor ativo na MESMA família (cenário "Renovação normal").
func TestRefreshRotation(t *testing.T) {
	sess, org := uuid.New(), uuid.New()
	now := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	exp := now.Add(2 * time.Hour)

	_, h1, _ := NewRefreshSecret()
	first, err := NewRefreshFamily(sess, org, h1, exp)
	if err != nil {
		t.Fatalf("NewRefreshFamily: %v", err)
	}
	if !first.Usable(now) {
		t.Fatalf("o primeiro token deveria ser usável")
	}

	_, h2, _ := NewRefreshSecret()
	second, err := first.Rotate(h2, exp)
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if first.Status != RefreshRotated {
		t.Fatalf("o token anterior deveria ficar rotated, veio %s", first.Status)
	}
	if second.Status != RefreshActive || second.FamilyID != first.FamilyID {
		t.Fatalf("o sucessor deveria ser ativo na mesma família: %+v", second)
	}
	if second.ID == first.ID {
		t.Fatalf("o sucessor deveria ter id próprio")
	}

	// Rotacionar um token já rotacionado não é permitido.
	if _, err := first.Rotate(h2, exp); !errors.Is(err, ErrRefreshNotActive) {
		t.Fatalf("rotacionar rotated: err = %v, quero ErrRefreshNotActive", err)
	}
}

// Reuso detectado: apresentar um token rotacionado (ou revogado) sinaliza reuso
// (cenário "Reuso detectado" — a revogação da família é T-008).
func TestRefreshReuseDetection(t *testing.T) {
	sess, org := uuid.New(), uuid.New()
	now := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	exp := now.Add(2 * time.Hour)

	_, h1, _ := NewRefreshSecret()
	first, _ := NewRefreshFamily(sess, org, h1, exp)

	// Token ativo: sem reuso.
	if err := first.CheckReuse(); err != nil {
		t.Fatalf("token ativo não deveria sinalizar reuso: %v", err)
	}

	// Após rotação, o antigo é rotated → apresentá-lo é reuso.
	_, h2, _ := NewRefreshSecret()
	_, _ = first.Rotate(h2, exp)
	if err := first.CheckReuse(); !errors.Is(err, ErrRefreshReuse) {
		t.Fatalf("token rotacionado deveria sinalizar reuso: %v", err)
	}

	// Um token revogado também.
	first.Status = RefreshRevoked
	if err := first.CheckReuse(); !errors.Is(err, ErrRefreshReuse) {
		t.Fatalf("token revogado deveria sinalizar reuso: %v", err)
	}
}

// Um token expirado não é usável (independente do reuso).
func TestRefreshExpiry(t *testing.T) {
	sess, org := uuid.New(), uuid.New()
	now := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	_, h, _ := NewRefreshSecret()
	tok, _ := NewRefreshFamily(sess, org, h, now.Add(time.Hour))
	if tok.Usable(now.Add(2 * time.Hour)) {
		t.Fatalf("token expirado não deveria ser usável")
	}
}
