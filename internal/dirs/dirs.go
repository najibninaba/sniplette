package dirs

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

const appName = "sniplette"

// AppName returns the canonical application name for directory paths.
func AppName() string {
	return appName
}

// ConfigDir returns the app's configuration directory.
// - Linux: $XDG_CONFIG_HOME/sniplette or ~/.config/sniplette
// - macOS: ~/Library/Application Support/sniplette
// - Windows: %AppData%/sniplette (fallback to os.UserConfigDir)
func ConfigDir() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", AppName()), nil
	case "linux":
		xdg := os.Getenv("XDG_CONFIG_HOME")
		if xdg != "" {
			return filepath.Join(xdg, AppName()), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".config", AppName()), nil
	default:
		// Windows and other OSes fall back to UserConfigDir
		cfg, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(cfg, AppName()), nil
	}
}

// DataDir returns the app's data directory.
// - Linux: $XDG_DATA_HOME/sniplette or ~/.local/share/sniplette
// - macOS: ~/Library/Application Support/sniplette
// - Windows: %AppData%/sniplette (fallback to os.UserConfigDir)
func DataDir() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", AppName()), nil
	case "linux":
		xdg := os.Getenv("XDG_DATA_HOME")
		if xdg != "" {
			return filepath.Join(xdg, AppName()), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".local", "share", AppName()), nil
	default:
		// Windows and other OSes fall back to UserConfigDir as a reasonable place
		cfg, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(cfg, AppName()), nil
	}
}

// CacheDir returns the app's cache directory.
// - Linux: $XDG_CACHE_HOME/sniplette or ~/.cache/sniplette
// - macOS: ~/Library/Caches/sniplette
// - Windows: %LocalAppData%/sniplette (fallback to os.UserCacheDir)
func CacheDir() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Caches", AppName()), nil
	case "linux":
		xdg := os.Getenv("XDG_CACHE_HOME")
		if xdg != "" {
			return filepath.Join(xdg, AppName()), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".cache", AppName()), nil
	default:
		// Windows and others via UserCacheDir
		c, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(c, AppName()), nil
	}
}

// StateDir returns the app's state directory.
// - Linux: $XDG_STATE_HOME/sniplette or ~/.local/state/sniplette
// - macOS: ~/Library/Application Support/sniplette/state
// - Windows: %LocalAppData%/sniplette/state (fallback to ConfigDir/state)
func StateDir() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", AppName(), "state"), nil
	case "linux":
		xdg := os.Getenv("XDG_STATE_HOME")
		if xdg != "" {
			return filepath.Join(xdg, AppName()), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".local", "state", AppName()), nil
	default:
		// Windows and others: try LocalAppData, else fall back under config
		if la := os.Getenv("LOCALAPPDATA"); la != "" {
			return filepath.Join(la, AppName(), "state"), nil
		}
		cfg, err := ConfigDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(cfg, "state"), nil
	}
}

// DefaultOutputDir returns the default output directory under the data dir.
func DefaultOutputDir() (string, error) {
	d, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "output"), nil
}

// TempBaseDir returns the base directory for temporary working files under cache.
func TempBaseDir() (string, error) {
	c, err := CacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(c, "temp"), nil
}

// Ensure creates the directory if it doesn't exist.
func Ensure(path string) error {
	if path == "" {
		return errors.New("empty path")
	}
	return os.MkdirAll(path, 0o755)
}

// EnsureAll ensures config, data, cache, and state dirs exist.
func EnsureAll() error {
	if p, err := ConfigDir(); err == nil {
		if err := Ensure(p); err != nil {
			return err
		}
	}
	if p, err := DataDir(); err == nil {
		if err := Ensure(p); err != nil {
			return err
		}
	}
	if p, err := CacheDir(); err == nil {
		if err := Ensure(p); err != nil {
			return err
		}
	}
	if p, err := StateDir(); err == nil {
		if err := Ensure(p); err != nil {
			return err
		}
	}
	return nil
}