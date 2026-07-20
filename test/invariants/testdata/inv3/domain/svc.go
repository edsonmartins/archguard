// Fixture de teste-do-teste: pacote "de domínio" importando framework web,
// proibido por INV-3/ADR-0016. Vive em testdata/ e NÃO participa do build.
package domain

import (
	"github.com/beego/beego/v2/server/web"
)

var _ = web.BConfig
