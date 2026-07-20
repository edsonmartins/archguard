// Fixture de teste-do-teste: reproduz mutações proibidas por INV-2.
// Este arquivo vive em testdata/ e NÃO participa do build.
package fixture

type Record struct{ ID int64 }

type session interface {
	Exec(sql string, args ...interface{}) error
	Delete(bean interface{}) (int64, error)
}

func tamper(s session) {
	_ = s.Exec("DELETE FROM record WHERE id = ?", 1)
	_ = s.Exec("UPDATE record SET action = ? WHERE id = ?", "x", 1)
	_, _ = s.Delete(&Record{ID: 1})
}
