package prefs

import (
	"errors"
	"testing"

	"zakki/internal/atomicfile"
)

func TestCommittedWriteErrorAdvancesWrittenRevision(t *testing.T) {
	s := NewStore(t.TempDir()+"/preferences.json", 0)
	wantErr := errors.New("directory sync failed")
	writes := 0
	s.writeJSON = func(path string, value any) error {
		writes++
		if err := atomicfile.WriteJSON(path, value); err != nil {
			return err
		}
		return &atomicfile.CommitError{Err: wantErr}
	}
	snap := s.TakeSnapshot(Default())

	err := s.WriteSnapshot(snap)
	if !atomicfile.IsCommitted(err) || !errors.Is(err, wantErr) {
		t.Fatalf("首次保存错误 = %v, want committed error", err)
	}
	if s.writtenRev != snap.Rev {
		t.Fatalf("已写修订号 = %d, want %d", s.writtenRev, snap.Rev)
	}
	if s.LastSaveError() == "" {
		t.Fatal("目录同步失败必须如实记录")
	}

	if err := s.WriteSnapshot(snap); err != nil {
		t.Fatalf("同一已提交修订不应重复写入: %v", err)
	}
	if writes != 1 {
		t.Fatalf("写入次数 = %d, want 1", writes)
	}

	s.writeJSON = atomicfile.WriteJSON
	p := Default()
	p.Appearance.BodySize = 18
	if err := s.SaveNow(p); err != nil {
		t.Fatal(err)
	}
	if s.LastSaveError() != "" {
		t.Fatalf("后续新修订成功后应清除错误: %q", s.LastSaveError())
	}
}
