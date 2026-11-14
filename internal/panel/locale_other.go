//go:build !darwin || !cgo

package panel

// SystemLocale returns the UI locale matched from POSIX locale environment variables.
func SystemLocale() Locale {
	return envLocale()
}
