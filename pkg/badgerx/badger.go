package badgerx

import (
	"os"
	"path/filepath"

	rbadger "github.com/leafney/rose-badger"
)

type BadgerSvc struct {
	*rbadger.BadgerDB
}

// NewBadgerSvc 创建 BadgerDB 服务（Agent 端使用）
// dbPath: 数据库文件路径
// stop: 停止信号通道
func NewBadgerSvc(dbPath string, stop chan struct{}) (*BadgerSvc, error) {
	// 确保目录存在
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	// 打开数据库
	db, err := rbadger.NewBadgerDB(dbPath)
	if err != nil {
		return nil, err
	}

	// 监听停止信号，优雅关闭
	if stop != nil {
		go func() {
			<-stop
			_ = db.Close()
		}()
	}

	return &BadgerSvc{db}, nil
}

// NewDefaultBadgerSvc 创建默认的 BadgerDB 服务
func NewDefaultBadgerSvc(dbPath string) (*BadgerSvc, error) {
	return NewBadgerSvc(dbPath, nil)
}
