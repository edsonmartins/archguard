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

// Package alerting holds PROVISIONAL implementations of the domain.Alerter port.
//
// These are NOT the production control: real alerting flows to the
// observability/paging pipeline (ADR-0013, pacote 010). The provisional ones
// log and/or record alerts so the scheduled verification (T-015) and other
// callers are usable and testable now.
package alerting

import (
	"context"
	"log"
	"sync"

	"github.com/casdoor/casdoor/internal/domain"
)

// LogAlerter writes each alert to the standard logger — a dev/CI sink. It never
// puts personal data in the log because domain.Alert carries none.
type LogAlerter struct{}

// NewLogAlerter builds the logging alerter.
func NewLogAlerter() *LogAlerter { return &LogAlerter{} }

// Alert logs the alert.
func (a *LogAlerter) Alert(_ context.Context, alert domain.Alert) error {
	log.Printf("[ALERT %s] %s: %s", alert.Severity, alert.Subject, alert.Detail)
	return nil
}

var _ domain.Alerter = (*LogAlerter)(nil)

// MemoryAlerter records alerts in memory, for inspection in tests.
type MemoryAlerter struct {
	mu     sync.Mutex
	alerts []domain.Alert
}

// NewMemoryAlerter builds an empty in-memory alerter.
func NewMemoryAlerter() *MemoryAlerter { return &MemoryAlerter{} }

// Alert records the alert.
func (a *MemoryAlerter) Alert(_ context.Context, alert domain.Alert) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.alerts = append(a.alerts, alert)
	return nil
}

// Alerts returns a copy of the recorded alerts.
func (a *MemoryAlerter) Alerts() []domain.Alert {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]domain.Alert, len(a.alerts))
	copy(out, a.alerts)
	return out
}

var _ domain.Alerter = (*MemoryAlerter)(nil)
