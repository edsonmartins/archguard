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

package alerting

import (
	"context"
	"testing"

	"github.com/casdoor/casdoor/internal/domain"
)

func TestMemoryAlerterRecords(t *testing.T) {
	a := NewMemoryAlerter()
	if err := a.Alert(context.Background(), domain.Alert{Severity: domain.SeverityCritical, Subject: "x", Detail: "y"}); err != nil {
		t.Fatalf("Alert: %v", err)
	}
	got := a.Alerts()
	if len(got) != 1 || got[0].Severity != domain.SeverityCritical || got[0].Subject != "x" {
		t.Fatalf("alerta não registrado: %+v", got)
	}
}

func TestLogAlerterDoesNotError(t *testing.T) {
	if err := NewLogAlerter().Alert(context.Background(), domain.Alert{Severity: domain.SeverityInfo, Subject: "s"}); err != nil {
		t.Fatalf("LogAlerter: %v", err)
	}
}
