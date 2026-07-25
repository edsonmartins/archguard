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

package object

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strings"

	"github.com/beego/beego/v2/server/web"
	"github.com/casdoor/casdoor/conf"
	"github.com/casdoor/casdoor/internal/migrate"
	"github.com/casdoor/casdoor/util"
	xormadapter "github.com/casdoor/xorm-adapter/v3"
	_ "github.com/lib/pq" // db = postgres — único backend suportado (ADR-0009)
	"github.com/xorm-io/xorm"
	"github.com/xorm-io/xorm/core"
	"github.com/xorm-io/xorm/names"
)

const (
	defaultConfigPath     = "conf/app.conf"
	defaultExportFilePath = "init_data_dump.json"
)

var (
	ormer          *Ormer = nil
	createDatabase        = true
	configPath            = defaultConfigPath
	exportData            = false
	exportFilePath        = defaultExportFilePath
)

func InitFlag() {
	createDatabasePtr := flag.Bool("createDatabase", false, "true if you need to create database")
	configPathPtr := flag.String("config", defaultConfigPath, "set it to \"/your/path/app.conf\" if your config file is not in: \"/conf/app.conf\"")
	exportDataPtr := flag.Bool("export", false, "export database to JSON file and exit (use -exportPath to specify custom location)")
	exportFilePathPtr := flag.String("exportPath", defaultExportFilePath, "path to the exported data file (used with -export)")
	flag.Parse()

	createDatabase = *createDatabasePtr
	configPath = *configPathPtr
	exportData = *exportDataPtr
	exportFilePath = *exportFilePathPtr

	// Load beego config from the specified config path
	err := web.LoadAppConfig("ini", configPath)
	if err != nil {
		panic(fmt.Sprintf("failed to load config from %s: %v", configPath, err))
	}
}

func ShouldExportData() bool {
	return exportData
}

func GetExportFilePath() string {
	return exportFilePath
}

func InitConfig() {
	err := web.LoadAppConfig("ini", "../conf/app.conf")
	if err != nil {
		panic(err)
	}

	web.BConfig.WebConfig.Session.SessionOn = true

	InitAdapter()
	CreateTables()
}

func InitAdapter() {
	// ADR-0009: PostgreSQL 15+ é o único backend suportado. Recusa iniciar com
	// qualquer outro dialeto, de forma explícita (spec fork-baseline).
	if driverName := conf.GetConfigString("driverName"); driverName != "" && driverName != "postgres" {
		panic(fmt.Sprintf("unsupported database driver: %q — ArchGuard supports only PostgreSQL 15+ (driverName must be \"postgres\", ADR-0009)", driverName))
	}
	if conf.GetConfigString("driverName") == "" {
		if !util.FileExist(configPath) {
			dir, err := os.Getwd()
			if err != nil {
				panic(err)
			}
			dir = strings.ReplaceAll(dir, "\\", "/")
			panic(fmt.Sprintf("The Casdoor config file: \"app.conf\" was not found, it should be placed at: \"%s/conf/app.conf\"", dir))
		}
	}

	if createDatabase {
		err := createDatabaseForPostgres(conf.GetConfigString("driverName"), conf.GetConfigDataSourceName(), conf.GetConfigString("dbName"))
		if err != nil {
			panic(err)
		}
	}

	var err error
	ormer, err = NewAdapter(conf.GetConfigString("driverName"), conf.GetConfigDataSourceName(), conf.GetConfigString("dbName"))
	if err != nil {
		panic(err)
	}

	tableNamePrefix := conf.GetConfigString("tableNamePrefix")
	tbMapper := names.NewPrefixMapper(names.SnakeMapper{}, tableNamePrefix)
	ormer.Engine.SetTableMapper(tbMapper)
}

func CreateTables() {
	if createDatabase {
		err := ormer.CreateDatabase()
		if err != nil {
			panic(err)
		}
	}

	ormer.createTable()
}

// RunMigrations applies the versioned, explicit SQL migrations (ADR-0009) after
// the legacy XORM Sync2 has created/updated the tables. Destructive schema
// changes (e.g. dropping columns of removed scope) live here, never in Sync2.
func RunMigrations() {
	// Role segregation (ADR-0009, deploy/postgres/roles.sql): migrations run as
	// archguard_migrate (DDL) via migrationDataSourceName when configured; the
	// runtime application connects as archguard_app (which lacks UPDATE/DELETE on
	// audit tables). Falls back to the app DSN when not segregated (e.g. dev).
	dsn := conf.GetConfigString("migrationDataSourceName")
	if dsn == "" {
		dsn = conf.GetConfigDataSourceName()
	}
	if err := migrate.Run(context.Background(), dsn); err != nil {
		panic(err)
	}
}

// Ormer represents the MySQL adapter for policy storage.
type Ormer struct {
	driverName     string
	dataSourceName string
	dbName         string
	Db             *sql.DB
	Engine         *xorm.Engine
}

// finalizer is the destructor for Ormer.
func finalizer(a *Ormer) {
	if a.Engine != nil {
		err := a.Engine.Close()
		if err != nil {
			panic(err)
		}
		a.Engine = nil
	}

	if a.Db != nil {
		err := a.Db.Close()
		if err != nil {
			panic(err)
		}
		a.Db = nil
	}
}

// NewAdapter is the constructor for Ormer.
func NewAdapter(driverName string, dataSourceName string, dbName string) (*Ormer, error) {
	a := &Ormer{}
	a.driverName = driverName
	a.dataSourceName = dataSourceName
	a.dbName = dbName

	// Open the DB, create it if not existed.
	err := a.open()
	if err != nil {
		return nil, err
	}

	// Call the destructor when the object is released.
	runtime.SetFinalizer(a, finalizer)

	return a, nil
}

// NewAdapterFromDb is the constructor for Ormer.
func NewAdapterFromDb(driverName string, dataSourceName string, dbName string, db *sql.DB) (*Ormer, error) {
	a := &Ormer{}
	a.driverName = driverName
	a.dataSourceName = dataSourceName
	a.dbName = dbName
	a.Db = db

	// Open the DB, create it if not existed.
	err := a.openFromDb(a.Db)
	if err != nil {
		return nil, err
	}

	// Call the destructor when the object is released.
	runtime.SetFinalizer(a, finalizer)

	return a, nil
}

func refineDataSourceNameForPostgres(dataSourceName string) string {
	reg := regexp.MustCompile(`dbname=[^ ]+\s*`)
	return reg.ReplaceAllString(dataSourceName, "dbname=postgres")
}

func createDatabaseForPostgres(driverName string, dataSourceName string, dbName string) error {
	if driverName == "postgres" {
		db, err := sql.Open(driverName, refineDataSourceNameForPostgres(dataSourceName))
		if err != nil {
			return err
		}
		defer db.Close()

		_, err = db.Exec(fmt.Sprintf("CREATE DATABASE \"%s\";", dbName))
		if err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return err
			}
		}
		schema := util.GetValueFromDataSourceName("search_path", dataSourceName)
		if schema != "" {
			db, err = sql.Open(driverName, dataSourceName)
			if err != nil {
				return err
			}
			defer db.Close()

			_, err = db.Exec(fmt.Sprintf("CREATE SCHEMA %s;", schema))
			if err != nil {
				if !strings.Contains(err.Error(), "already exists") {
					return err
				}
			}
		}

		return nil
	} else {
		return nil
	}
}

func (a *Ormer) CreateDatabase() error {
	// PostgreSQL databases are created out-of-band (createDatabaseForPostgres);
	// nothing to do here for the sole supported backend (ADR-0009).
	return nil
}

func (a *Ormer) open() error {
	dataSourceName := a.dataSourceName
	engine, err := xorm.NewEngine("postgres", dataSourceName)
	if err != nil {
		return err
	}

	schema := util.GetValueFromDataSourceName("search_path", dataSourceName)
	if schema != "" {
		engine.SetSchema(schema)
	}

	a.Engine = engine
	return nil
}

func (a *Ormer) openFromDb(db *sql.DB) error {
	dataSourceName := a.dataSourceName

	xormDb := core.FromDB(db)

	engine, err := xorm.NewEngineWithDB("postgres", dataSourceName, xormDb)
	if err != nil {
		return err
	}

	schema := util.GetValueFromDataSourceName("search_path", dataSourceName)
	if schema != "" {
		engine.SetSchema(schema)
	}

	a.Engine = engine
	return nil
}

func (a *Ormer) close() {
	runtime.SetFinalizer(a, nil)
	finalizer(a)
}

// createTable sincroniza o esquema das tabelas legadas do Casdoor. Usa Sync (ADITIVO:
// cria tabela/coluna/índice que falta) em vez de Sync2 (DESTRUTIVO: dropa índices do
// DB ausentes no struct). Motivo: as migrations do ArchGuard (internal/migrate)
// adicionam colunas/índices (id estável, chaves de RLS) às tabelas gerenciadas pelo
// XORM; o Sync2 tentaria dropar esses índices a CADA boot e panicava no restart
// (nome de DROP reconstruído incorreto). Sync não dropa — o boot fica restart-safe.
// A evolução destrutiva de esquema é responsabilidade das migrations, não do Sync.
func (a *Ormer) createTable() {
	showSql := conf.GetConfigBool("showSql")
	a.Engine.ShowSQL(showSql)

	err := a.Engine.Sync(new(Organization))
	if err != nil {
		panic(err)
	}

	err = a.Engine.Sync(new(Group))
	if err != nil {
		panic(err)
	}

	err = a.Engine.Sync(new(User))
	if err != nil {
		panic(err)
	}

	err = a.Engine.Sync(new(Invitation))
	if err != nil {
		panic(err)
	}

	err = a.Engine.Sync(new(Application))
	if err != nil {
		panic(err)
	}

	err = a.Engine.Sync(new(Provider))
	if err != nil {
		panic(err)
	}

	err = a.Engine.Sync(new(Resource))
	if err != nil {
		panic(err)
	}

	err = a.Engine.Sync(new(Cert))
	if err != nil {
		panic(err)
	}

	err = a.Engine.Sync(new(Key))
	if err != nil {
		panic(err)
	}

	err = a.Engine.Sync(new(Role))
	if err != nil {
		panic(err)
	}

	err = a.Engine.Sync(new(Permission))
	if err != nil {
		panic(err)
	}

	err = a.Engine.Sync(new(Model))
	if err != nil {
		panic(err)
	}

	err = a.Engine.Sync(new(Adapter))
	if err != nil {
		panic(err)
	}

	err = a.Engine.Sync(new(Enforcer))
	if err != nil {
		panic(err)
	}

	err = a.Engine.Sync(new(Session))
	if err != nil {
		panic(err)
	}

	err = a.Engine.Sync(new(Token))
	if err != nil {
		panic(err)
	}

	err = a.Engine.Sync(new(Transaction))
	if err != nil {
		panic(err)
	}

	err = a.Engine.Sync(new(Coupon))
	if err != nil {
		panic(err)
	}

	err = a.Engine.Sync(new(CouponUsage))
	if err != nil {
		panic(err)
	}

	err = a.Engine.Sync(new(Syncer))
	if err != nil {
		panic(err)
	}

	err = a.Engine.Sync(new(Record))
	if err != nil {
		panic(err)
	}

	err = a.Engine.Sync(new(Webhook))
	if err != nil {
		panic(err)
	}

	err = a.Engine.Sync(new(WebhookEvent))
	if err != nil {
		panic(err)
	}

	err = a.Engine.Sync(new(VerificationRecord))
	if err != nil {
		panic(err)
	}

	err = a.Engine.Sync(new(Ldap))
	if err != nil {
		panic(err)
	}

	err = a.Engine.Sync(new(RadiusAccounting))
	if err != nil {
		panic(err)
	}

	err = a.Engine.Sync(new(xormadapter.CasbinRule))
	if err != nil {
		panic(err)
	}

	err = a.Engine.Sync(new(Form))
	if err != nil {
		panic(err)
	}

	err = a.Engine.Sync(new(Ticket))
	if err != nil {
		panic(err)
	}

	err = a.Engine.Sync(new(Entry))
	if err != nil {
		panic(err)
	}

	err = a.Engine.Sync(new(Site))
	if err != nil {
		panic(err)
	}

	err = a.Engine.Sync(new(Rule))
	if err != nil {
		panic(err)
	}

	err = a.Engine.Sync(new(ThirdPartyLink))
	if err != nil {
		panic(err)
	}
}
