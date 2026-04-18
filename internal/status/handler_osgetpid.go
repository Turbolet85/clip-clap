package status

import "os"

// realOSGetpid is the default pid-getter. Separated so handler_test.go
// can override osGetpid (via a test helper) without taking a direct
// dependency on the os package at the package-level var declaration.
func realOSGetpid() int {
	return os.Getpid()
}
