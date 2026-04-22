//go:build !linux && !windows

package helpers

func IsAdmin() bool {
	return true
}
