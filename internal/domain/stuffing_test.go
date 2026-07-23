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
	"testing"
	"time"

	"github.com/google/uuid"
)

// Muitas identidades distintas de UMA origem na janela → alerta; o alerta dispara
// UMA vez (cenário "Padrão distribuído").
func TestStuffingDetectorAlertsOnDistributedPattern(t *testing.T) {
	d := NewStuffingDetector()
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	origin := "origin-hash-A"

	// As primeiras (threshold-1) identidades não alertam.
	for i := 0; i < stuffingDistinctThreshold-1; i++ {
		alert, distinct := d.Observe(origin, uuid.New(), now)
		if alert {
			t.Fatalf("não deveria alertar com %d identidades distintas", distinct)
		}
	}
	// A que atinge o limiar alerta.
	alert, distinct := d.Observe(origin, uuid.New(), now)
	if !alert || distinct != stuffingDistinctThreshold {
		t.Fatalf("deveria alertar no limiar: alert=%v distinct=%d", alert, distinct)
	}
	// Observações subsequentes não repetem o alerta na mesma janela.
	if alert2, _ := d.Observe(origin, uuid.New(), now); alert2 {
		t.Fatalf("não deveria realertar na mesma janela")
	}
}

// A mesma identidade repetida NÃO conta como distinta.
func TestStuffingDetectorCountsDistinctOnly(t *testing.T) {
	d := NewStuffingDetector()
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	id := uuid.New()
	for i := 0; i < stuffingDistinctThreshold+5; i++ {
		alert, distinct := d.Observe("origin-B", id, now)
		if alert || distinct != 1 {
			t.Fatalf("uma única identidade não deveria alertar: alert=%v distinct=%d", alert, distinct)
		}
	}
}

// Origens diferentes são independentes: espalhar por várias origens não alerta.
func TestStuffingDetectorPerOrigin(t *testing.T) {
	d := NewStuffingDetector()
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	for i := 0; i < stuffingDistinctThreshold+5; i++ {
		origin := "origin-" + uuid.New().String()
		if alert, _ := d.Observe(origin, uuid.New(), now); alert {
			t.Fatalf("origens distintas não deveriam alertar")
		}
	}
}

// Entradas fora da janela são podadas: identidades antigas não contam, e após a
// janela drenar a origem pode alertar de novo.
func TestStuffingDetectorWindowPruning(t *testing.T) {
	d := NewStuffingDetector()
	base := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	origin := "origin-C"

	// Enche até um a menos do limiar.
	for i := 0; i < stuffingDistinctThreshold-1; i++ {
		d.Observe(origin, uuid.New(), base)
	}
	// Muito depois da janela, uma nova identidade encontra as antigas podadas →
	// distinct volta a 1, sem alerta.
	later := base.Add(2 * stuffingWindow)
	alert, distinct := d.Observe(origin, uuid.New(), later)
	if alert || distinct != 1 {
		t.Fatalf("entradas antigas deveriam ser podadas: alert=%v distinct=%d", alert, distinct)
	}
}
