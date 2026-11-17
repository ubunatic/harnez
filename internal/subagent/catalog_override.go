package subagent

import "ubunatic.com/harnez/modelcatalog"

// UseModelCatalogForTest swaps the merged model catalog and returns a restore
// function. Tests use it to opt into shipped-disabled models (Jules, issue
// 1242) without a user models.yaml; production code must not call it.
func UseModelCatalogForTest(catalog *modelcatalog.Catalog) (restore func()) {
	if err := ensureModelCatalog(); err != nil {
		panic(err)
	}
	old := mergedModelCatalog
	mergedModelCatalog = catalog
	return func() { mergedModelCatalog = old }
}
