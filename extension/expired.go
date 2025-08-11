package extension

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/walterwanderley/sqlite"

	cachehttp "github.com/walterwanderley/sqlite-http-cache/http"
)

type Expired struct{}

func (m *Expired) Args() int           { return 3 }
func (m *Expired) Deterministic() bool { return true }
func (m *Expired) Apply(ctx *sqlite.Context, values ...sqlite.Value) {
	var header http.Header
	if err := json.Unmarshal([]byte(values[0].Text()), &header); err != nil {
		ctx.ResultNull()
		return
	}

	var timestamp *time.Time
	if !values[1].IsNil() {
		if v, err := time.Parse(time.RFC3339Nano, values[1].Text()); err != nil {
			ctx.ResultError(fmt.Errorf("invalid 'timestamp' param: %v", err))
			return
		} else {
			timestamp = &v
		}
	}

	var shared bool
	if v, err := strconv.ParseBool(values[2].Text()); err != nil {
		ctx.ResultError(fmt.Errorf("invalid 'shared' param: %v", err))
		return
	} else {
		shared = v
	}

	cacheControl := cachehttp.ParseCacheControl(header, timestamp, shared)
	ctx.ResultText(strconv.FormatBool(cacheControl.Expired()))
}
