//go:build wireinject
// +build wireinject

package wire

import (
	"github.com/google/wire"
	"github.com/leafney/filedock/core"
)

func InitializeApp(configPath string, build core.BuildInfo) (*core.App, error) {
	wire.Build(
		provideConfig,
		provideLogger,
		provideSQLiteDB,
		provideVersionSvc,
		provideVersionBiz,
		provideVersionAPI,
		provideServer,
		provideApp,
	)
	return nil, nil
}
