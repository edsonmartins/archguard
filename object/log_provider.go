// Copyright 2026 The Casdoor Authors. All Rights Reserved.
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

package object

import (
	"fmt"
	"sync"

	"github.com/casdoor/casdoor/log"
)

var (
	runningCollectors   = map[string]log.LogProvider{} // providerGetId() -> LogProvider
	runningCollectorsMu sync.Mutex
)

// InitLogProviders scans all globally-configured Log providers and starts
// background collection for pull-based providers (System Log, SELinux Log).
// It is called once from main() after the database is ready.
func InitLogProviders() {
	providers, err := GetGlobalProviders()
	if err != nil {
		return
	}
	for _, p := range providers {
		if p.Category != "Log" {
			continue
		}
		if p.State == "Disabled" {
			continue
		}
		switch p.Type {
		case "System Log", "SELinux Log":
			startLogCollector(p)
		}
	}
}

func stopCollector(id string) {
	runningCollectorsMu.Lock()
	defer runningCollectorsMu.Unlock()

	if existing, ok := runningCollectors[id]; ok {
		_ = existing.Stop()
		delete(runningCollectors, id)
	}
}

// startLogCollector starts a pull-based log collector (System Log / SELinux Log)
// for the given provider. If a collector for the same provider is already
// running it is stopped first.
func startLogCollector(provider *Provider) {
	id := provider.GetId()
	stopCollector(id)

	lp, err := log.GetLogProvider(provider.Type, provider.Host, provider.Port, provider.Title)
	if err != nil {
		return
	}

	providerName := provider.Name
	addEntry := func(owner, createdTime, _ string, message string) error {
		name := log.GenerateEntryName()
		entry := &Entry{
			Owner:       owner,
			Name:        name,
			CreatedTime: createdTime,
			UpdatedTime: createdTime,
			DisplayName: name,
			Provider:    providerName,
			Message:     message,
		}
		_, err := AddEntry(entry)
		return err
	}

	onError := func(err error) {
		fmt.Printf("InitLogProviders: collector for provider %s stopped with error: %v\n", providerName, err)
	}
	if err := lp.Start(addEntry, onError); err != nil {
		fmt.Printf("InitLogProviders: failed to start collector for provider %s: %v\n", providerName, err)
		return
	}

	runningCollectorsMu.Lock()
	defer runningCollectorsMu.Unlock()
	runningCollectors[id] = lp
}

func refreshLogProviderRuntime(oldID string, provider *Provider) {
	if provider == nil {
		if oldID != "" {
			stopLogProviderRuntime(oldID)
		}
		return
	}
	if oldID != "" {
		stopLogProviderRuntime(oldID)
	}
	if provider.Category != "Log" {
		return
	}
	if provider.State == "Disabled" {
		return
	}

	switch provider.Type {
	case "System Log", "SELinux Log":
		startLogCollector(provider)
	}
}

func stopLogProviderRuntime(providerID string) {
	if providerID == "" {
		return
	}
	stopCollector(providerID)
}
