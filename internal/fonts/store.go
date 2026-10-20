package fonts

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

var (
	// ErrInvalidResource 表示资源元数据或生产下载白名单校验失败。
	ErrInvalidResource = errors.New("invalid font resource")
	// ErrCorruptCache 仅表示长度或校验和不符，不包含文件读取错误。
	ErrCorruptCache = errors.New("corrupt font cache")
)

// Store 不启动后台任务；并发调用由调用方决定，发布通过同目录 rename 完成。
type Store struct {
	Dir    string
	Client *http.Client
}

// Progress 回调在调用 Ensure 的线程同步执行，不具有 UI 线程语义。
type Progress struct {
	Downloaded, Total int64
	Stage             string
}

func validID(id string) bool {
	if len(id) == 0 || len(id) > 64 || id[0] < 'a' || id[0] > 'z' {
		return false
	}
	for _, c := range id {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

// Path 仅使用安全 ASCII ID，不使用家族名；目录未配置时返回空。
func (s Store) Path(r Resource) string {
	if s.Dir == "" || !validID(r.ID) {
		return ""
	}
	return filepath.Join(s.Dir, r.ID+".ttf")
}

func validMetadata(r Resource) bool {
	digest, err := hex.DecodeString(r.SHA256)
	return validID(r.ID) && r.Bytes > 0 && r.Bytes < 1<<63-1 && err == nil && len(digest) == sha256.Size
}

func verify(r Resource, data []byte) error {
	if int64(len(data)) != r.Bytes {
		return fmt.Errorf("%w: length got %d, want %d", ErrCorruptCache, len(data), r.Bytes)
	}
	digest := sha256.Sum256(data)
	expected, _ := hex.DecodeString(r.SHA256)
	if !bytes.Equal(digest[:], expected) {
		return fmt.Errorf("%w: checksum mismatch", ErrCorruptCache)
	}
	return nil
}

func emit(report func(Progress), stage string, downloaded, total int64) {
	if report != nil {
		report(Progress{Downloaded: downloaded, Total: total, Stage: stage})
	}
}

// Read 校验缓存但不修复；调用方可用 errors.Is 区分缺失、损坏与 I/O 错误。
func (s Store) Read(r Resource) ([]byte, error) {
	return s.read(r, nil)
}

func (s Store) read(r Resource, report func(Progress)) ([]byte, error) {
	path := s.Path(r)
	if path == "" || !validMetadata(r) {
		return nil, ErrInvalidResource
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open font cache: %w", err)
	}
	data, readErr := io.ReadAll(io.LimitReader(f, r.Bytes+1))
	closeErr := f.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return nil, fmt.Errorf("read font cache: %w", err)
	}
	emit(report, "verifying", int64(len(data)), r.Bytes)
	if err := verify(r, data); err != nil {
		return nil, err
	}
	return data, nil
}

// Ensure 只接受与目录完全相同的资源，不能通过替换 URL 或校验值绕过白名单。
func (s Store) Ensure(ctx context.Context, r Resource, report func(Progress)) ([]byte, error) {
	known, ok := Find(r.ID)
	if !ok || known != r {
		return nil, ErrInvalidResource
	}
	return s.ensure(ctx, r, report)
}

// ensure 留作包内离线测试接缝；生产入口必须先核对固定目录。
func (s Store) ensure(ctx context.Context, r Resource, report func(Progress)) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.Path(r) == "" || !validMetadata(r) {
		return nil, ErrInvalidResource
	}
	data, err := s.read(r, report)
	if err == nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		emit(report, "ready", r.Bytes, r.Bytes)
		return data, nil
	}
	if !errors.Is(err, os.ErrNotExist) && !errors.Is(err, ErrCorruptCache) {
		return nil, err
	}
	return s.download(ctx, r, report)
}

func (s Store) download(ctx context.Context, r Resource, report func(Progress)) (_ []byte, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("font request: %w", err)
	}
	client := http.Client{Timeout: 2 * time.Minute}
	if s.Client != nil {
		client = *s.Client
		if client.Timeout == 0 {
			client.Timeout = 2 * time.Minute
		}
	}
	response, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download font: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download font: HTTP %d", response.StatusCode)
	}
	if response.ContentLength >= 0 && response.ContentLength != r.Bytes {
		return nil, fmt.Errorf("download font: length got %d, want %d", response.ContentLength, r.Bytes)
	}
	if err := os.MkdirAll(s.Dir, 0700); err != nil {
		return nil, fmt.Errorf("create font directory: %w", err)
	}
	f, err := os.CreateTemp(s.Dir, ".font-*")
	if err != nil {
		return nil, fmt.Errorf("create font staging file: %w", err)
	}
	closed := false
	defer func() {
		if !closed {
			err = errors.Join(err, f.Close())
		}
		if removeErr := os.Remove(f.Name()); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			err = errors.Join(err, removeErr)
		}
	}()
	var data bytes.Buffer
	emit(report, "downloading", 0, r.Bytes)
	reader := io.LimitReader(response.Body, r.Bytes+1)
	buf := make([]byte, 32*1024)
	var downloaded int64
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		n, readErr := reader.Read(buf)
		if n > 0 {
			downloaded += int64(n)
			if downloaded > r.Bytes {
				return nil, fmt.Errorf("download font: exceeds %d bytes", r.Bytes)
			}
			if _, err := f.Write(buf[:n]); err != nil {
				return nil, fmt.Errorf("write font staging file: %w", err)
			}
			data.Write(buf[:n])
			emit(report, "downloading", downloaded, r.Bytes)
		}
		if readErr != nil {
			if !errors.Is(readErr, io.EOF) {
				return nil, fmt.Errorf("read font response: %w", readErr)
			}
			break
		}
	}
	emit(report, "verifying", downloaded, r.Bytes)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := verify(r, data.Bytes()); err != nil {
		return nil, fmt.Errorf("download font: %w", err)
	}
	if err := f.Sync(); err != nil {
		return nil, fmt.Errorf("sync font staging file: %w", err)
	}
	closed = true
	if err := f.Close(); err != nil {
		return nil, fmt.Errorf("close font staging file: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := os.Rename(f.Name(), s.Path(r)); err != nil {
		return nil, fmt.Errorf("publish font cache: %w", err)
	}
	emit(report, "ready", downloaded, r.Bytes)
	return data.Bytes(), nil
}
