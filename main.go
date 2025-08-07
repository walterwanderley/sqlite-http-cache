package main

import (
	_ "github.com/walterwanderley/sqlite-http-cache/extension"
)

// #cgo linux LDFLAGS: -Wl,--unresolved-symbols=ignore-in-object-files
// #cgo darwin LDFLAGS: -Wl,-undefined,dynamic_lookup
// #cgo windows LDFLAGS: -Wl,--allow-multiple-definition
import "C"

func main() {

}
