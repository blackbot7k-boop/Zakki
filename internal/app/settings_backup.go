package app

// 设置页「备份与数据」：备份文件夹卡片按所选文件夹的形态展示——普通文件夹
// （本地或云盘同步盘）只做自动备份；文件夹是 git 仓库时追加 Git 自动提交与
// 推送区块。数据文件入口与持久化状态也归本页。

import (
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"zakki/internal/notes"
	"zakki/locales"
)

// ---- 备份与数据 ----

func (a *app) backupPage(c *ui.Context, t *ui.Theme, dark bool) {
	ui.Column(c).FillWidth().Children(func() {
		// 未保存指示与写保护状态。
		a.persistStatus(c, t)

		ui.Scroll(c).Fill().Grow(1).ScrollbarInsets(0, 12, 0, 0).Children(func() {
			ui.Column(c).FillWidth().Padding(0, 22, 0, 0).Children(func() {
				settingsSectionTitle(c, locales.CardBackupFolder())
				settingsLead(c, locales.BackupFolderHint())
				mono := monoFontStack("sfmono")
				// 备份文件夹是唯一存储地址：未选择时显示正在使用的内置目录。
				chosen := a.prefs.Backup.BackupDir
				dir := a.notes.BackupDir()
				ui.Column(c).FillWidth().PaddingY(8).Gap(6).Children(func() {
					if chosen == "" {
						ui.Text(c, locales.BackupBuiltinLead()).FontSize(12).TextColor(tMuted(c))
					}
					ui.Text(c, dir).FontSize(12).Font(mono)
					ui.Row(c).Gap(8).Children(func() {
						if ui.Button(c, locales.OpenFolder()).Clicked() {
							_ = mygo.Shell.OpenPath(dir)
						}
						if chosen == "" {
							if ui.Button(c, locales.ChooseFolder()).Clicked() {
								a.pickBackupFolder()
							}
						} else {
							if ui.Button(c, locales.CommonChange()).Clicked() {
								a.pickBackupFolder()
							}
							if ui.Button(c, locales.CommonDisconnect()).Clicked() {
								a.disconnectBackupFolder()
							}
						}
					})
				})
				// 文件夹是 git 仓库时才展示 Git 自动化区块；普通文件夹保持原样。
				if a.settingsUI.gitProbeDir != dir {
					a.settingsUI.gitProbeDir, a.settingsUI.gitProbeRepo = dir, notes.DirIsGitRepo(dir)
				}
				if a.settingsUI.gitProbeRepo {
					a.gitBackupSection(c)
				}
				sectionDivider(c)
				settingsSectionTitle(c, locales.CardDataFile())
				settingsCompactRow(c, locales.ShowDataLocation(), a.notes.DataFile, func() {
					if ui.Button(c, locales.ShowInFinder()).Clicked() {
						mygo.Shell.ShowItemInFolder(a.notes.DataFile)
					}
				})
				sectionDivider(c)
				a.cardResetFooter(c, "backup", pageBackup)
			})

			// 本页操作错误（备份失败、目录不可用等）。
			if a.settingsUI.keyErr != "" {
				ui.Text(c, a.settingsUI.keyErr).FontSize(12).TextColor(t.Danger)
			}
		})
	})
}

// gitBackupSection Git 自动化区块：总开关、推送间隔与最近提交/推送状态。
// 只在备份文件夹探测到 .git 时出现；开关关掉后文件夹仍作普通备份用。
func (a *app) gitBackupSection(c *ui.Context) {
	sectionDivider(c)
	settingsSectionTitle(c, locales.GitSectionTitle())
	settingsCompactRow(c, locales.GitAutoPushLabel(), locales.GitAutoPushHint(), func() {
		// Switch 直接绑定草稿字段；Changed 在下一帧到货时值已写入草稿。
		if ui.Switch(c, &a.settingsUI.draft.Backup.GitAutoPush).Changed() {
			a.pushBackup()
		}
	})
	if a.settingsUI.draft.Backup.GitAutoPush {
		pushMin := float64(a.settingsUI.draft.Backup.GitPushMinutes)
		settingsCompactRow(c, locales.GitPushIntervalLabel(), locales.GitPushIntervalHint(), func() {
			if ui.NumberInput(c, &pushMin, 1, 1440, 1).Changed() {
				a.settingsUI.draft.Backup.GitPushMinutes = int(pushMin)
				a.pushBackup()
			}
		})
	}
	if status := a.gitStatusText(); status != "" {
		ui.Text(c, status).FontSize(11).TextColor(tMuted(c)).PaddingY(4)
	}
}

// gitStatusText Git 自动化状态行：最近提交/推送时间，有失败时附原因。
func (a *app) gitStatusText() string {
	commit, push := a.notes.GitCommitTime(), a.notes.GitPushTime()
	errText := a.notes.GitError()
	var parts []string
	if commit.IsZero() {
		parts = append(parts, locales.GitStatusNever())
	} else {
		parts = append(parts, locales.GitStatusCommit(commit.Format("01-02 15:04")))
	}
	if !push.IsZero() {
		parts = append(parts, locales.GitStatusPush(push.Format("01-02 15:04")))
	}
	if errText != "" {
		parts = append(parts, locales.GitStatusFailed(errText))
	}
	return strings.Join(parts, " · ")
}

// persistStatus 持久化状态：未保存指示、重试入口、写保护恢复。
func (a *app) persistStatus(c *ui.Context, t *ui.Theme) {
	if msg := a.prefsSaveError(); msg != "" {
		ui.Column(c).FillWidth().Gap(6).Padding(10, 12).Radius(8).
			Background(t.Surface).BorderWidth(1).BorderColor(t.Danger).Children(func() {
			ui.Text(c, locales.SettingsUnsaved()).Bold().TextColor(t.Danger)
			ui.Text(c, msg).FontSize(12).TextColor(tMuted(c))
			if ui.Button(c, locales.RetrySave()).Clicked() {
				a.retryPreferencesSave()
				a.settingsUI.syncDraftFromPrefs(a.prefs)
			}
		})
	}
	blocked := a.notes.SaveBlocked()
	if blocked {
		ui.Column(c).FillWidth().Gap(6).Padding(10, 12).Radius(8).
			Background(t.Surface).BorderWidth(1).BorderColor(t.Warning).Children(func() {
			ui.Text(c, locales.WriteProtectedTitle()).Bold()
			ui.Text(c, locales.WriteProtectedBody()).FontSize(12).TextColor(tMuted(c))
			if ui.Button(c, locales.WriteBackButton()).Clicked() {
				if err := a.confirmWriteBack(); err != nil {
					a.settingsUI.keyErr = locales.WriteBackFailed(err.Error())
				}
			}
		})
	}
}
