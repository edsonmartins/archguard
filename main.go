// Copyright 2021 The Casdoor Authors. All Rights Reserved.
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
//
// Modified by IntegrAllTech Ltda. — changes recorded in docs/upstream/DIVERGENCE.md.

package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/beego/beego/v2/core/logs"
	"github.com/beego/beego/v2/server/web"
	_ "github.com/beego/beego/v2/server/web/session/redis"
	"github.com/casdoor/casdoor/authz"
	"github.com/casdoor/casdoor/conf"
	"github.com/casdoor/casdoor/controllers"
	"github.com/casdoor/casdoor/internal/boot"
	"github.com/casdoor/casdoor/internal/deploy"
	"github.com/casdoor/casdoor/internal/domain"
	"github.com/casdoor/casdoor/object"
	"github.com/casdoor/casdoor/proxy"
	"github.com/casdoor/casdoor/radius"
	"github.com/casdoor/casdoor/routers"
	"github.com/casdoor/casdoor/service"
	"github.com/casdoor/casdoor/util"
)

func main() {
	web.BConfig.WebConfig.Session.SessionOn = true
	web.BConfig.WebConfig.Session.SessionName = "archguard_session_id"
	if conf.GetConfigString("redisEndpoint") == "" {
		web.BConfig.WebConfig.Session.SessionProvider = "file"
		web.BConfig.WebConfig.Session.SessionProviderConfig = "./tmp"
	} else {
		web.BConfig.WebConfig.Session.SessionProvider = "redis"
		web.BConfig.WebConfig.Session.SessionProviderConfig = conf.GetConfigString("redisEndpoint")
	}
	sessionCookieLifeTime := 3600 * 24 * 30
	if val, err := conf.GetConfigInt64("sessionCookieLifeTime"); err == nil && val > 0 {
		sessionCookieLifeTime = int(val)
	}
	web.BConfig.WebConfig.Session.SessionCookieLifeTime = sessionCookieLifeTime
	web.BConfig.WebConfig.Session.SessionGCMaxLifetime = int64(sessionCookieLifeTime)
	// web.BConfig.WebConfig.Session.SessionCookieSameSite = http.SameSiteNoneMode

	routers.InitAPI()
	object.InitFlag()
	object.InitDeploymentProfile()
	object.InitKeystore()
	object.InitAdapter()
	object.CreateTables()
	object.RunMigrations()

	object.InitDb()

	// Handle export command
	if object.ShouldExportData() {
		exportPath := object.GetExportFilePath()
		err := object.DumpToFile(exportPath)
		if err != nil {
			panic(fmt.Sprintf("Error exporting data to %s: %v", exportPath, err))
		}
		fmt.Printf("Data exported successfully to %s\n", exportPath)
		return
	}

	// Runtime pgx pool for the ArchGuard composition root (pacote 011, T-001).
	// Opened after migrations (the schema must exist) and closed on shutdown.
	// Fatal on failure: the composition root cannot serve without its database.
	if err := boot.InitPool(context.Background(), conf.GetConfigDataSourceName()); err != nil {
		panic(err)
	}
	defer boot.ClosePool()

	// Adapter factory (pacote 011, T-002/T-004b): selects adapters by deployment
	// profile and vends the key custodian (dev: keystore-backed provisional;
	// conformant: fail-closed until OpenBao). The dev keystore is nil outside dev.
	boot.InitFactory(deploy.Active(), boot.Pool(), object.DevKeystore())

	// Seed the built-in admin's domain identity + membership (pacote 011, T-004b)
	// so the console works for the inherited admin. Runs only where custody is
	// available (dev); conformant profiles skip it until OpenBao is wired. Idempotent
	// and dedup-respecting; a failure in dev is fatal (the console needs the admin).
	if custodian, cerr := boot.ActiveFactory().KeyCustodian(); cerr == nil {
		if serr := boot.SeedBuiltInAdmin(context.Background(), boot.Pool(), custodian, "built-in", "admin@example.com"); serr != nil {
			panic(fmt.Sprintf("seed do admin de domínio (T-004b) falhou: %v", serr))
		}
	} else {
		fmt.Printf("ArchGuard: custódia indisponível no perfil ativo — admin de domínio não semeado (%v)\n", cerr)
	}

	// Control-plane API mux (pacote 011, T-003). Built before capability handlers
	// are registered (T-005+); the Beego bridge delegates /api/v1/* to it.
	boot.InitAPIMux()

	// Assurance pipeline (pacote 011, T-004): resolves the session and enforces
	// operation classification (INV-8) for the mounted domain handlers. Fail-closed
	// until the login bridge (T-004b) establishes sessions.
	boot.InitPipeline(boot.Pool())

	// Mount the control-plane capability handlers onto /api/v1 (pacote 011, T-005+),
	// each wrapped by the assurance pipeline and classified (INV-8).
	if err := boot.MountCapabilities(); err != nil {
		panic(fmt.Sprintf("montagem das capacidades do control plane (pacote 011) falhou: %v", err))
	}

	object.InitDefaultStorageProvider()
	object.InitLogProviders()
	object.InitLdapAutoSynchronizer()
	proxy.InitHttpClient()
	authz.InitApi()
	object.InitUserManager()
	object.InitFromFile()
	object.SealCerts()
	object.InitCleanupTokens()
	object.InitCleanupDeviceAuthMap()

	object.InitSiteMap()
	if len(object.SiteMap) != 0 {
		object.InitRuleMap()
		object.StartMonitorSitesLoop()
	}

	util.SafeGoroutine(func() { object.RunSyncUsersJob() })
	util.SafeGoroutine(func() { controllers.InitCLIDownloader() })

	// web.DelStaticPath("/static")
	// web.SetStaticPath("/static", "web/build/static")

	web.BConfig.WebConfig.DirectoryIndex = true
	if web.BConfig.RunMode == "dev" {
		web.SetStaticPath("/swagger", "swagger")
	}
	web.SetStaticPath("/files", "files")
	// https://studygolang.com/articles/2303
	web.InsertFilter("*", web.BeforeStatic, routers.RequestBodyFilter)
	web.InsertFilter("*", web.BeforeStatic, routers.ContentTypeFilter)
	web.InsertFilter("*", web.BeforeRouter, routers.StaticFilter)
	web.InsertFilter("*", web.BeforeRouter, routers.AutoSigninFilter)
	web.InsertFilter("*", web.BeforeRouter, routers.CorsFilter)
	web.InsertFilter("*", web.BeforeRouter, routers.TimeoutFilter)
	web.InsertFilter("*", web.BeforeRouter, routers.ApiFilter)
	web.InsertFilter("*", web.BeforeRouter, routers.PrometheusFilter)
	web.InsertFilter("*", web.BeforeRouter, routers.RecordMessage)
	web.InsertFilter("*", web.BeforeRouter, routers.FieldValidationFilter)
	web.InsertFilter("*", web.AfterExec, routers.AfterRecordMessage, web.WithReturnOnOutput(false))

	var logAdapter string
	logConfigMap := make(map[string]interface{})
	err := json.Unmarshal([]byte(conf.GetConfigString("logConfig")), &logConfigMap)
	if err != nil {
		panic(err)
	}
	_, ok := logConfigMap["adapter"]
	if !ok {
		logAdapter = "file"
	} else {
		logAdapter = logConfigMap["adapter"].(string)
	}
	if logAdapter == "console" {
		logs.Reset()
	}
	err = logs.SetLogger(logAdapter, conf.GetConfigString("logConfig"))
	if err != nil {
		panic(err)
	}

	port := web.AppConfig.DefaultInt("httpport", 8000)
	// logs.SetLevel(logs.LevelInformational)
	logs.SetLogFuncCall(false)

	err = util.StopOldInstance(port)
	if err != nil {
		panic(err)
	}

	// Legacy edge channel (RFC-0007 §6 / ADR-0019): the embedded RADIUS server is
	// DISABLED BY DEFAULT (I-4.4). It starts only when explicitly enabled via
	// `enableRadiusServer`; absent/empty/false leaves it off. The embedded LDAP
	// server was removed (ADR-0019); the client LDAP/AD connector (pacote 009) is
	// separate and unaffected.
	if domain.NewLegacyChannelConfig(conf.GetConfigString("enableRadiusServer")).Enabled(domain.LegacyRADIUS) {
		go radius.StartRadiusServer()
	}
	go object.ClearThroughputPerSecond()

	// Start webhook delivery worker
	object.StartWebhookDeliveryWorker()

	if len(object.SiteMap) != 0 {
		service.Start()
	}

	web.Run(fmt.Sprintf(":%v", port))
}
