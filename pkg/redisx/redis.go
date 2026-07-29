package redisx

import (
	"fmt"

	"github.com/leafney/filedock/pkg/configx"
	"github.com/leafney/filedock/pkg/zlogx"
	rredis "github.com/leafney/rose-redis"
)

type RedisSvc struct {
	*rredis.Redis
	log *zlogx.ZLogSvc
}

func NewRedisSvc(cfg configx.RedisConfig, log *zlogx.ZLogSvc) (*RedisSvc, error) {
	client, err := rredis.NewRedis(cfg.GetRedisAddr(), &rredis.Option{
		Pass: cfg.GetRedisPassword(),
		DB:   cfg.GetRedisDB(),
		Type: rredis.TypeNode,
	})
	if err != nil {
		return nil, fmt.Errorf("redis connect error: %w", err)
	}

	ping := client.Ping()
	if !ping {
		_ = client.Close()
		return nil, fmt.Errorf("redis ping error")
	}

	log.Info("[Redis] Load successful")
	return &RedisSvc{Redis: client, log: log}, nil
}

func (s *RedisSvc) Close() error {
	if s == nil || s.Redis == nil {
		return nil
	}
	if err := s.Redis.Close(); err != nil {
		return err
	}
	if s.log != nil {
		s.log.Info("[Redis] Exit successful")
	}
	return nil
}
