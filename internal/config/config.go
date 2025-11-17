package config

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"ig2wa/internal/dirs"
)

// Init wires Viper with config paths, env, defaults, and flag bindings.
// It is non-fatal: any errors are returned for optional handling by caller.
func Init(root *cobra.Command) error {
	// Ensure base directories exist
	_ = dirs.EnsureAll()

	// Setup config search path
	if cfgDir, err := dirs.ConfigDir(); err == nil {
		_ = dirs.Ensure(cfgDir)
		viper.AddConfigPath(cfgDir)
	}
	viper.SetConfigName("config") // supports config.{yaml|yml|json|toml}

	// Environment variables: SNIPLETTE_*
	viper.SetEnvPrefix("SNIPLETTE")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	// Bind root persistent flags to Viper keys
	_ = viper.BindPFlag("out_dir", root.PersistentFlags().Lookup("out-dir"))
	_ = viper.BindPFlag("verbose", root.PersistentFlags().Lookup("verbose"))
	_ = viper.BindPFlag("dl_binary", root.PersistentFlags().Lookup("dl-binary"))
	_ = viper.BindPFlag("jobs", root.PersistentFlags().Lookup("jobs"))

	// Read config file if present (ignore not found)
	_ = viper.ReadInConfig()

	return nil
}