package app

import (
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"zakki/locales"
)

// storagePage 展示唯一笔记文件与独立的 Git 调度；渲染路径不做磁盘或仓库探测。
func (a *app) storagePage(c *ui.Context) {
	ui.Scroll(c.Key("storage-controls")).Fill().Children(func() {
		ui.Column(c).FillWidth().Padding(0, 22, 0, 0).Children(func() {
			a.persistStatus(c)
			settingsSectionTitle(c, locales.StorageFile())
			settingsLead(c, locales.StorageFileHint())
			ui.Text(c, a.notes.DataPath()).FontSize(12).Wrap()
			ui.Row(c).Gap(8).PaddingY(12).Children(func() {
				if ui.Button(c, locales.StorageShowFile()).Clicked() {
					mygo.Shell.ShowItemInFolder(a.notes.DataPath())
				}
				if ui.Button(c, locales.StorageChangeLocation()).Disabled(a.storageMoving).Clicked() {
					a.pickStorageFolder()
				}
			})
			settingsRow(c, locales.StorageInterval(), locales.StorageIntervalHint(), func() {
				seconds := float64(a.settingsUI.draft.Storage.SaveIntervalMS) / 1000
				if ui.NumberInput(c.Key("save-interval"), &seconds, 0.1, 60, 0.5).Width(150).Label(locales.StorageInterval()).Changed() {
					_ = a.updatePreferences(func(p *preferences) { p.Storage.SaveIntervalMS = int(seconds * 1000) })
				}
			})
			ui.Text(c, a.storageStatus()).FontSize(12).TextColor(tMuted(c))
			if ui.Button(c, locales.MenuSave()).Clicked() {
				a.saveCurrent()
			}
			sectionDivider(c)
			settingsSectionTitle(c, locales.StorageGit())
			status := a.notes.GitStatus()
			switch {
			case status.Missing:
				settingsLead(c, locales.GitToolsMissing())
			case status.RepoRoot == "":
				settingsLead(c, locales.StorageNoRepo())
			default:
				ui.Text(c, status.RepoRoot).FontSize(12).TextColor(tMuted(c))
			}
			settingsRow(c, locales.StorageAutoCommit(), locales.StorageCommitHint(), func() {
				if ui.Switch(c.Key("auto-commit"), &a.settingsUI.draft.Git.AutoCommit).Label(locales.StorageAutoCommit()).Changed() {
					enabled := a.settingsUI.draft.Git.AutoCommit
					_ = a.updatePreferences(func(p *preferences) {
						p.Git.AutoCommit = enabled
						if !enabled {
							p.Git.AutoPush = false
						}
					})
				}
			})
			if a.prefs.Git.AutoCommit {
				settingsRow(c, locales.StorageCommitInterval(), "", func() {
					minutes := float64(a.prefs.Git.CommitMinutes)
					if ui.NumberInput(c.Key("commit-minutes"), &minutes, 1, 1440, 1).Width(150).Label(locales.StorageCommitInterval()).Changed() {
						_ = a.updatePreferences(func(p *preferences) { p.Git.CommitMinutes = int(minutes) })
					}
				})
				settingsRow(c, locales.StorageAutoPush(), locales.StoragePushHint(), func() {
					if ui.Switch(c.Key("auto-push"), &a.settingsUI.draft.Git.AutoPush).Label(locales.StorageAutoPush()).Changed() {
						enabled := a.settingsUI.draft.Git.AutoPush
						_ = a.updatePreferences(func(p *preferences) { p.Git.AutoPush = enabled })
					}
				})
				if a.prefs.Git.AutoPush {
					settingsRow(c, locales.StoragePushInterval(), "", func() {
						minutes := float64(a.prefs.Git.PushMinutes)
						if ui.NumberInput(c.Key("push-minutes"), &minutes, 1, 1440, 1).Width(150).Label(locales.StoragePushInterval()).Changed() {
							_ = a.updatePreferences(func(p *preferences) { p.Git.PushMinutes = int(minutes) })
						}
					})
				}
			}
			for _, event := range []struct {
				label string
				at    time.Time
			}{
				{locales.StorageLastCommit(), status.CommitTime}, {locales.StorageLastPush(), status.PushTime},
			} {
				if !event.at.IsZero() {
					ui.Text(c, event.label+" "+event.at.Format("01-02 15:04")).FontSize(11).TextColor(tMuted(c))
				}
			}
			if status.Error != "" {
				ui.Text(c, status.Error).FontSize(12).TextColor(c.Theme().Danger)
			}
			sectionDivider(c)
			a.cardResetFooter(c, "storage", pageStorage)
		})
	})
}

func (a *app) persistStatus(c *ui.Context) {
	if msg := a.prefsSaveError(); msg != "" {
		ui.Column(c).FillWidth().Gap(6).PaddingY(8).Children(func() {
			ui.Text(c, msg).TextColor(c.Theme().Danger)
			if ui.Button(c, locales.RetrySave()).Clicked() {
				a.retryPreferencesSave()
			}
		})
	}
	if msg := a.storageFailure(); msg != "" {
		ui.Column(c).FillWidth().Gap(6).PaddingY(8).Children(func() {
			ui.Text(c, msg).TextColor(c.Theme().Danger)
			if a.notes.SaveBlocked() {
				if ui.Button(c, locales.StorageReload()).Clicked() {
					a.reloadNotes()
				}
			} else if ui.Button(c, locales.RetrySave()).Clicked() {
				a.saveCurrent()
			}
		})
	}
	if a.storageMoving {
		ui.Text(c, locales.StorageMoving()).TextColor(tMuted(c))
	}
}
