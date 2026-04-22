//go:build linux

package helpers

import "os"

func IsAdmin() bool {
	return os.Geteuid() == 0
}
