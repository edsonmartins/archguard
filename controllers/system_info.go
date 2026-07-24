// Copyright 2022 The Casdoor Authors. All Rights Reserved.
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

package controllers

import (
	"errors"

	"github.com/casdoor/casdoor/internal/deploy"
	"github.com/casdoor/casdoor/internal/domain"
	"github.com/casdoor/casdoor/util"
	"github.com/go-git/go-git/v5"
)

// GetSystemInfo
// @Title GetSystemInfo
// @Tag System API
// @Description get system info like CPU and memory usage
// @Success 200 {object} util.SystemInfo The Response object
// @router /get-system-info [get]
func (c *ApiController) GetSystemInfo() {
	_, ok := c.RequireAdmin()
	if !ok {
		return
	}

	systemInfo, err := util.GetSystemInfo()
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(systemInfo)
}

// GetVersionInfo
// @Title GetVersionInfo
// @Tag System API
// @Description get version info like Casdoor release version and commit ID
// @Success 200 {object} util.VersionInfo The Response object
// @router /get-version-info [get]
func (c *ApiController) GetVersionInfo() {
	versionInfo, err := util.GetVersionInfo()
	if err != nil && !errors.Is(err, git.ErrRepositoryNotExists) {
		c.ResponseError(err.Error())
		return
	}

	if versionInfo.Version != "" {
		c.ResponseOk(versionInfo)
		return
	}

	c.ResponseOk(util.GetBuiltInVersionInfo())
}

// Health
// @Title Health
// @Tag System API
// @Description check if the system is live; reports the deployment profile,
// key custodian and compliance (ADR-0017 §4)
// @Success 200 {object} controllers.Response The Response object
// @router /health [get]
func (c *ApiController) Health() {
	profile := deploy.Active()
	// Compliance is CUSTODY-based (pacote 010, T-016): local key custody is never
	// production-conformant, even outside the dev profile — the private material
	// must live in the vault (ADR-0012). The health check flags it unmistakably.
	custody := domain.CustodyVault
	if profile.KeyCustodian() != "openbao" {
		custody = domain.CustodyLocal
	}
	report := domain.ComplianceReport{Custody: custody, ProfileConformant: profile.Conformant()}
	c.ResponseOk(map[string]any{
		"status":            "ok",
		"profile":           string(profile),
		"keyCustodian":      profile.KeyCustodian(),
		"compliance":        report.Status(),
		"nonConformReasons": report.Reasons(),
	})
}
