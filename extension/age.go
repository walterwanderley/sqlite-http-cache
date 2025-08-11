package extension

import (
	"encoding/json"
	"net/http"

	"github.com/walterwanderley/sqlite"

	cachehttp "github.com/walterwanderley/sqlite-http-cache/http"
)

type Age struct{}

func (m *Age) Args() int           { return 2 }
func (m *Age) Deterministic() bool { return true }
func (m *Age) Apply(ctx *sqlite.Context, values ...sqlite.Value) {
	var header http.Header
	if err := json.Unmarshal([]byte(values[0].Text()), &header); err != nil {
		ctx.ResultNull()
		return
	}
	// TODO utilizar segundo parametro para os casos que o header nao tem o date
	age := cachehttp.Age(header)
	if age == nil {
		ctx.ResultNull()
		return
	}

	ctx.ResultInt(*age)
}
