package extension

import (
	"github.com/walterwanderley/sqlite"
	"github.com/walterwanderley/sqlite-http-cache/config"
)

// #cgo CFLAGS: -DSQLITE_CORE
// #cgo linux LDFLAGS: -Wl,--unresolved-symbols=ignore-in-object-files
// #cgo darwin LDFLAGS: -Wl,-undefined,dynamic_lookup
// #cgo windows LDFLAGS: -Wl,--allow-multiple-definition
//
// #include "sqlite3.h"
//
// extern int sqlite3_extension_init(sqlite3*, char**, const sqlite3_api_routines*);
import "C"

func init() {
	sqlite.RegisterNamed("default", func(api *sqlite.ExtensionApi) (sqlite.ErrorCode, error) {
		if err := api.CreateModule(config.DefaultVirtualTableName, &CacheModule{}, sqlite.ReadOnly(false)); err != nil {
			return sqlite.SQLITE_ERROR, err
		}
		if err := api.CreateFunction("cacheage", &Age{}); err != nil {
			return sqlite.SQLITE_ERROR, err
		}
		if err := api.CreateFunction("cachelifetime", &FreshnessLifetime{}); err != nil {
			return sqlite.SQLITE_ERROR, err
		}
		if err := api.CreateFunction("cachexpired", &Expired{}); err != nil {
			return sqlite.SQLITE_ERROR, err
		}
		return sqlite.SQLITE_OK, nil
	})
	C.sqlite3_auto_extension((*[0]byte)(C.sqlite3_extension_init))
}
