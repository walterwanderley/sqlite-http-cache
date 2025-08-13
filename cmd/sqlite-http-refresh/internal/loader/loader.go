package loader

import _ "github.com/walterwanderley/sqlite-http-cache/extension"

// #cgo CFLAGS: -DSQLITE_CORE
// #cgo linux LDFLAGS: -Wl,--unresolved-symbols=ignore-in-object-files
// #cgo darwin LDFLAGS: -Wl,-undefined,dynamic_lookup
// #cgo windows LDFLAGS: -Wl,--allow-multiple-definition
//
// #include "../../../../sqlite3.h"
//
// extern int sqlite3_extension_init(sqlite3*, char**, const sqlite3_api_routines);
import "C"

func init() {
	C.sqlite3_auto_extension((*[0]byte)(C.sqlite3_extension_init))
}
