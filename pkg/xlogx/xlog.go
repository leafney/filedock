/**
 * @Author:      leafney
 * @GitHub:      https://github.com/leafney
 * @Project:     grape
 * @Date:        2024-10-20 15:06
 * @Description:
 */

package xlogx

import (
	"github.com/leafney/rose/xlog"
)

type XLogSvc struct {
	*xlog.Log
}

// NewXLogSvc 创建日志服务
// debug: 是否开启调试模式
// enable: 是否启用日志
// level: 日志级别（debug/info/warn/error）
func NewXLogSvc(debug bool, enable bool, level string) *XLogSvc {
	x := xlog.NewXLog()
	// debug model
	x.SetDebug(debug)
	// enable
	x.SetEnable(enable)
	// default log level
	x.SetLevelStr(level)

	x.Infoln("[XLog] Load successful")

	return &XLogSvc{x}
}
