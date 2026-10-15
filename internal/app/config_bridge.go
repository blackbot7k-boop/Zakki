package app

import (
	"time"

	"tot-md-mygo/internal/legacyconfig"
)

type fileConfig = legacyconfig.Config
type appearanceConfig = legacyconfig.Appearance
type editorConfig = legacyconfig.Editor
type windowConfig = legacyconfig.Window
type shortcutConfig = legacyconfig.Shortcuts
type storageConfig = legacyconfig.Storage

func defaultFileConfig(s settings) legacyconfig.Config {
	return legacyconfig.Default(notesBaselineFromSettings(s), int(backupInterval/time.Minute))
}

func decodeConfig(raw []byte, defaults legacyconfig.Config) (legacyconfig.Config, error) {
	return legacyconfig.DecodeTOML(raw, defaults)
}

func decodeLegacyConfig(raw []byte, defaults legacyconfig.Config) (legacyconfig.Config, error) {
	return legacyconfig.DecodeJSON(raw, defaults)
}
