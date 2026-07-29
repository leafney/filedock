/**
 * @Author:      leafney
 * @GitHub:      https://github.com/leafney
 * @Project:     smart-life-assistant
 * @Date:        2025-09-09
 * @Description: 公共配置接口定义，用于解耦pkg包与具体配置实现
 */

package configx

// BadgerDBConfig BadgerDB配置接口
type BadgerDBConfig interface {
	GetBadgerDBPath() string
}

// CacheConfig 缓存配置接口
type CacheConfig interface {
	GetCacheMinutes() int64
}

// LogConfig 日志配置接口
type LogConfig interface {
	GetLogLevel() string
	GetLogFormat() string
	IsDebugMode() bool
}

// RabbitMQConfig RabbitMQ配置接口
type RabbitMQConfig interface {
	GetRMQAddr() string
	GetRMQExchange() string
	GetRMQQueue() string
	IsRMQDebugMode() bool
}

// RedisConfig Redis配置接口
type RedisConfig interface {
	GetRedisAddr() string
	GetRedisPassword() string
	GetRedisDB() int
}

// MongoConfig MongoDB配置接口
type MongoConfig interface {
	GetMongoURI() string
	GetMongoDatabase() string
}

// ServerSpecConfig 服务规格配置接口
type ServerSpecConfig interface {
	GetMaxTasks() int
	GetTaskTimeout() int
	GetCacheExpiration() int
}

// ClientSpecConfig 客户端规格配置接口
type ClientSpecConfig interface {
	GetMaxConcurrentTasks() int
	GetSyncInterval() int
	GetCacheExpiration() int
}

// WorkerSpecConfig Worker规格配置接口
type WorkerSpecConfig interface {
	GetMaxConcurrentTasks() int
	GetTaskRetryCount() int
	GetHeartbeatInterval() int
}

// ScriptsConfig 脚本配置接口
type ScriptsConfig interface {
	GetScriptsPath() string
	GetNodePath() string
	GetScriptTimeout() int
}

// JWTConfig JWT配置接口
type JWTConfig interface {
	GetJWTSigningKey() string
	GetJWTExpireHours() int
}

// ServerAPIConfig 服务端API配置接口
type ServerAPIConfig interface {
	GetServerBaseURL() string
	GetServerTimeout() int
}

// S3Config S3 对象存储配置接口
type S3Config interface {
	GetS3Endpoint() string
	GetS3AccessKey() string
	GetS3SecretKey() string
	GetS3Bucket() string
	GetS3UseSSL() bool
}
