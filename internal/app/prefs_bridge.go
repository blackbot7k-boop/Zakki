package app

import (
	"time"

	"tot-md-mygo/internal/legacyconfig"
	"tot-md-mygo/internal/prefs"
)

type preferences = prefs.Preferences
type appearancePrefs = prefs.Appearance
type windowPrefs = prefs.Window
type backupPrefs = prefs.Backup
type keymapPrefs = prefs.Keymap
type advancedPrefs = prefs.Advanced
type migrationResult = prefs.MigrationResult
type prefsIssue = prefs.Issue
type prefsIssueKind = prefs.IssueKind

const (
	prefsIssueNone         = prefs.IssueNone
	prefsIssueLegacyBroken = prefs.IssueLegacyBroken
	prefsIssuePrefsBroken  = prefs.IssuePrefsBroken
)

func defaultPreferences() preferences {
	return prefs.Default(int(backupInterval / time.Minute))
}

func validatePrefsAccel(acc string) error {
	if acc == "" {
		return nil
	}
	_, err := parseAccelerator(acc)
	return err
}

func validatePrefs(p preferences) error { return p.Validate(validatePrefsAccel) }

func clonePrefs(p preferences) preferences { return p.Clone() }

func decodePreferences(raw []byte) (preferences, error) {
	return prefs.Decode(raw, validatePrefsAccel)
}

func notesBaselineFromSettings(s settings) legacyconfig.NotesBaseline {
	return legacyconfig.NotesBaseline{
		ThemeSource: s.ThemeSource, MarkdownTheme: s.MarkdownTheme,
		BodyFont: s.BodyFont, MonoFont: s.MonoFont,
		BodySize: s.BodySize, LineHeight: s.LineHeight,
		AlwaysOnTop: s.AlwaysOnTop, HideOnEscape: s.HideOnEscape,
	}
}

func migrateLegacyConfig(dir string, base settings) (preferences, migrationResult) {
	return prefs.MigrateLegacy(dir, notesBaselineFromSettings(base),
		int(backupInterval/time.Minute), cmdToggleWindow, migrateToggleWindow)
}

func newPrefsStore(prefsPath string) *prefs.Store {
	return prefs.NewStore(prefsPath, 500*time.Millisecond)
}

func migrateToggleWindow(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	if b, err := parseAccelerator(raw); err == nil {
		return b.accelerator(), nil
	}
	return normalizeLegacyAccelerator(raw)
}
