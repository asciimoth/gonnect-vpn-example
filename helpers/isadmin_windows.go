//go:build windows

package helpers

import "os"

func IsAdmin() bool {
	_, err := os.Open("\\\\.\\PHYSICALDRIVE0")
	return err == nil || !os.IsPermission(err)
}
