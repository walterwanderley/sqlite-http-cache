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
// extern int sqlite3_extension_init(sqlite3*, char**, const sqlite3_api_routines);
import "C"

func init() {
	sqlite.Register(registerFunc)
	C.sqlite3_auto_extension((*[0]byte)(C.sqlite3_extension_init))
}

func registerFunc(api *sqlite.ExtensionApi) (sqlite.ErrorCode, error) {
	if err := api.CreateModule(config.DefaultVirtualTableName, &CacheModule{}, sqlite.ReadOnly(false)); err != nil {
		return sqlite.SQLITE_ERROR, err
	}
	if err := api.CreateFunction("cache_info", &Info{}); err != nil {
		return sqlite.SQLITE_ERROR, err
	}
	if err := api.CreateFunction("cache_age", &Age{}); err != nil {
		return sqlite.SQLITE_ERROR, err
	}
	if err := api.CreateFunction("cache_lifetime", &FreshnessLifetime{}); err != nil {
		return sqlite.SQLITE_ERROR, err
	}
	if err := api.CreateFunction("cache_lifetime_shared", &FreshnessLifetime{
		shared: true,
	}); err != nil {
		return sqlite.SQLITE_ERROR, err
	}
	if err := api.CreateFunction("cache_expired", &Expired{}); err != nil {
		return sqlite.SQLITE_ERROR, err
	}
	if err := api.CreateFunction("cache_expired_ttl", &Expired{
		fallbackTTL: true,
	}); err != nil {
		return sqlite.SQLITE_ERROR, err
	}
	return sqlite.SQLITE_OK, nil
}
