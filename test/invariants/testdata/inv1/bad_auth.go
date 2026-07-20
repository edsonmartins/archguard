// Fixture de teste-do-teste: reproduz o padrão proibido por INV-1.
// Este arquivo vive em testdata/ e NÃO participa do build.
package fixture

type org struct{ MasterPassword string }

func authenticate(o org, password string) bool {
	if o.MasterPassword != "" && password == o.MasterPassword {
		return true // caminho proibido: credencial que não é do usuário
	}
	return false
}
