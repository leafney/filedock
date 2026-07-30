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
		provideI18nCatalog,
		provideSQLiteDB,
		provideVersionSvc,
		provideVersionBiz,
		provideVersionAPI,
		provideNicknameSvc,
		provideSessionSvc,
		provideSessionBiz,
		provideSessionAPI,
		provideRoomSvc,
		provideRoomBiz,
		provideRoomAPI,
		provideStreamHub,
		providePresenceSvc,
		provideStreamAPI,
		provideLifecycleSvc,
		provideRateLimiter,
		provideServer,
		provideApp,
	)
	return nil, nil
}
