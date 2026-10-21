package app

// 数据目录解析：正式安装用 <系统应用数据>/Zakki，开发运行用 Zakki Dev，
// 两边互不污染。应用不迁移旧目录：升级一律按全新安装处理。

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/egoist/mygo"
)

const (
	dataDirectoryName    = "Zakki"
	devDataDirectoryName = "Zakki Dev"
	dataFileName         = "zakki.json"
)

var (
	migrateDataDirOnce = new(sync.Once)
	resolvedDataDir    string
	resolvedDataDirErr error
	// devMode 判定开发运行；测试可替换。
	devMode = mygo.IsDev
)

// mygoAppDataBase 返回系统应用数据根目录；测试可替换。
var mygoAppDataBase = func() (string, error) { return mygo.App.Path(mygo.PathAppData) }

// resolveDataDir 只解析一次并缓存目录。生产启动必须检查 error；失败时本进程
// 不得构造 Store，下一次进程启动会重新尝试。
func resolveDataDir() (string, error) {
	migrateDataDirOnce.Do(func() {
		base, err := mygoAppDataBase()
		if err != nil || base == "" {
			base, _ = os.UserConfigDir()
		}
		resolvedDataDir, resolvedDataDirErr = resolveDataDirAt(base, devMode())
	})
	return resolvedDataDir, resolvedDataDirErr
}

// resolveDataDirAt 是纯解析逻辑；Once 之内不可重入，测试直接驱动它。
func resolveDataDirAt(base string, dev bool) (string, error) {
	if base == "" {
		return "", os.ErrNotExist
	}
	if dev {
		return filepath.Join(base, devDataDirectoryName), nil
	}
	return filepath.Join(base, dataDirectoryName), nil
}
