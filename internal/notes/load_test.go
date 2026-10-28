package notes

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"zakki/internal/atomicfile"
)

func doc(text string) File {
	return File{Version: CurrentDataVersion, Tabs: defaultTabs(text), Settings: Settings{ThemeSource: "system", HideOnEscape: true}}
}

func TestLoadSingleFileAndNoOp(t *testing.T) {
	s := newTestStore(t, nil)
	if _, exists, _, err := s.Load(); err != nil || exists {
		t.Fatalf("首次读取：%v %v", exists, err)
	}
	if err := s.SaveNow(doc("所有标签")); err != nil {
		t.Fatal(err)
	}
	fresh := NewStore(s.DataPath(), s.now, testMigrateOpt())
	defer fresh.Close()
	f, exists, hadTabs, err := fresh.Load()
	if err != nil || !exists || !hadTabs {
		t.Fatalf("读取：%v hadTabs=%v %v", exists, hadTabs, err)
	}
	fresh.stateWrite = func(string, []byte) error { t.Error("相同内容重复写盘"); return nil }
	if err := fresh.SaveNow(f); err != nil {
		t.Fatal(err)
	}
	if fresh.Unsaved() {
		t.Fatal("相同内容应已保存")
	}
	if !fresh.LastSaveTime().IsZero() {
		t.Fatal("无实际写入不更新时间")
	}
	entries, err := os.ReadDir(filepath.Dir(s.DataPath()))
	if err != nil || len(entries) != 1 || entries[0].Name() != "zakki.json" {
		t.Fatalf("不应产生副本：%v %v", entries, err)
	}
}

func TestLoadDiscardsFailedRetry(t *testing.T) {
	s := newTestStore(t, nil)
	defer s.Close()
	if err := s.SaveNow(doc("ORIGINAL")); err != nil {
		t.Fatal(err)
	}
	s.stateWrite = func(string, []byte) error { return errors.New("写入失败") }
	s.ScheduleSave(doc("LOCAL-STALE"), time.Hour, nil)
	s.mu.Lock()
	gen := s.timerGen
	s.mu.Unlock()
	s.flushPending(gen)
	s.mu.Lock()
	if s.pending == nil || s.saveTimer == nil {
		s.mu.Unlock()
		t.Fatal("失败后应等待重试")
	}
	oldGen := s.timerGen
	s.mu.Unlock()
	writeRaw(t, s.DataPath(), `{"version":3,"tabs":[{"id":1,"text":"EXTERNAL"}]}`)
	f, exists, _, err := s.Load()
	if err != nil || !exists || f.Tabs[0].Text != "EXTERNAL" {
		t.Fatalf("重读结果：%+v %v", f, err)
	}
	if s.Unsaved() {
		t.Fatal("成功重读后不应保留旧未保存快照")
	}
	s.mu.Lock()
	pending, timer := s.pending, s.saveTimer
	s.mu.Unlock()
	if pending != nil || timer != nil {
		t.Fatal("成功重读后必须撤销旧重试")
	}
	s.stateWrite = atomicfile.WriteBytes
	s.flushPending(oldGen)
	if got := readFile(t, s.DataPath()).Tabs[0].Text; got != "EXTERNAL" {
		t.Fatalf("旧重试覆盖外部内容：%q", got)
	}
}

func TestLoadBrokenOriginalCannotBeOverridden(t *testing.T) {
	for _, raw := range []string{`{"broken":`, `{"version":999,"tabs":[]}`} {
		t.Run(raw, func(t *testing.T) {
			s := newTestStore(t, nil)
			writeRaw(t, s.DataPath(), raw)
			if _, ok, _, err := s.Load(); ok || err == nil {
				t.Fatalf("损坏读取：%v %v", ok, err)
			}
			s.SetSaveBlocked(false)
			if !s.SaveBlocked() {
				t.Fatal("不得绕过损坏保护")
			}
			if err := s.SaveNow(doc("覆盖")); err == nil || !s.Unsaved() {
				t.Fatal("应拒写并保留未保存状态")
			}
			if got := string(readRaw(t, s.DataPath())); got != raw {
				t.Fatal("原件变化")
			}
			entries, _ := os.ReadDir(filepath.Dir(s.DataPath()))
			if len(entries) != 1 {
				t.Fatalf("不应生成副本：%v", entries)
			}
		})
	}
}

func TestExternalChangeAndDeletionRefuseSave(t *testing.T) {
	for _, remove := range []bool{false, true} {
		t.Run(map[bool]string{false: "修改", true: "丢失"}[remove], func(t *testing.T) {
			s := newTestStore(t, nil)
			if err := s.SaveNow(doc("原件")); err != nil {
				t.Fatal(err)
			}
			if remove {
				if err := os.Remove(s.DataPath()); err != nil {
					t.Fatal(err)
				}
			} else {
				writeRaw(t, s.DataPath(), `{"external":true}`)
			}
			if err := s.SaveNow(doc("原件")); !errors.Is(err, ErrExternalChange) {
				t.Fatalf("相同内容也应检查冲突：%v", err)
			}
			if err := s.Shutdown(doc("退出")); !errors.Is(err, ErrExternalChange) {
				t.Fatalf("退出不得覆盖：%v", err)
			}
			if remove {
				if _, ok, _, err := s.Load(); ok || !errors.Is(err, ErrExternalChange) {
					t.Fatalf("丢失后读取：%v %v", ok, err)
				}
				if _, err := os.Stat(s.DataPath()); !os.IsNotExist(err) {
					t.Fatal("不能重建")
				}
			} else if got := string(readRaw(t, s.DataPath())); got != `{"external":true}` {
				t.Fatal("不能覆盖")
			}
		})
	}
}

func TestReadExistingWithoutLoadRefusesOverwrite(t *testing.T) {
	s := newTestStore(t, nil)
	writeRaw(t, s.DataPath(), `{"version":3,"tabs":[]}`)
	if err := s.SaveNow(doc("未读不能覆盖")); !errors.Is(err, ErrExternalChange) {
		t.Fatalf("%v", err)
	}
}

func TestParseRawHideOnEscapeDefaults(t *testing.T) {
	s := newTestStore(t, nil)
	for _, tc := range []struct {
		raw  string
		want bool
	}{
		{`{"version":3,"tabs":[{"id":1,"text":"x"}]}`, true},
		{`{"version":3,"tabs":[{"id":1,"text":"x"}],"settings":{"hideOnEscape":false}}`, false},
	} {
		f, has, err := s.ParseRaw([]byte(tc.raw))
		if err != nil || !has || f.Settings.HideOnEscape != tc.want {
			t.Fatalf("%+v %v %v", f, has, err)
		}
	}
}

func TestLoadReportsHadTabs(t *testing.T) {
	s := newTestStore(t, nil)
	defer s.Close()
	writeRaw(t, s.DataPath(), `{"version":1,"active":0,"settings":{}}`)
	if _, exists, hadTabs, err := s.Load(); err != nil || !exists || hadTabs {
		t.Fatalf("无标签文件: exists=%v hadTabs=%v err=%v", exists, hadTabs, err)
	}
	writeRaw(t, s.DataPath(), `{"version":3,"active":0,"tabs":[{"id":1,"text":"内容"}]}`)
	if _, exists, hadTabs, err := s.Load(); err != nil || !exists || !hadTabs {
		t.Fatalf("有标签文件: exists=%v hadTabs=%v err=%v", exists, hadTabs, err)
	}
}
