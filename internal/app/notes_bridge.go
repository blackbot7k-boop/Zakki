package app

import (
	"time"

	"zakki/internal/notes"
)

type settings = notes.Settings
type savedTab = notes.Tab
type savedFile = notes.File

const currentDataVersion = notes.CurrentDataVersion

type versionError = notes.VersionError

func migrateSavedFile(f savedFile) (savedFile, error) {
	return notes.Migrate(f, notes.MigrateOptions{
		DefaultTabCount: DefaultTabCount,
		MaxTabCount:     MaxTabCount,
		PaletteColors:   append([]string(nil), tabPaletteColors[:]...),
		ValidTabColor:   validTabColor,
	})
}

func hasHideOnEscapeKey(raw []byte) bool { return notes.HasHideOnEscapeKey(raw) }

func notesMigrateOptions() notes.MigrateOptions {
	return notes.MigrateOptions{
		DefaultTabCount: DefaultTabCount,
		MaxTabCount:     MaxTabCount,
		PaletteColors:   append([]string(nil), tabPaletteColors[:]...),
		ValidTabColor:   validTabColor,
	}
}

func newNotesStore(dataFile string, now func() time.Time) *notes.Store {
	return notes.NewStore(dataFile, now, notesMigrateOptions())
}
