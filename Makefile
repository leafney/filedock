.DEFAULT_GOAL := help

BIN_DIR := bin
FRONTEND_DIR := frontend
STATIC_DIST_DIR := static/dist
CONFIG := data/config.toml
BIN := filedock
BIN_LIN := $(BIN)_linux_amd64
BIN_LIN_ARM := $(BIN)_linux_arm64

VERSION ?= $(shell git describe --tags --always 2>/dev/null || git rev-parse --short HEAD 2>/dev/null || echo "dev")
GIT_BRANCH ?= $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date +"%Y-%m-%d %H:%M:%S")

LDFLAGS := -s -w \
	-X 'main.Version=$(VERSION)' \
	-X 'main.GitBranch=$(GIT_BRANCH)' \
	-X 'main.GitCommit=$(GIT_COMMIT)' \
	-X 'main.BuildTime=$(BUILD_TIME)'

.PHONY: help clean deps dev web run build build-lin build-lin-arm air wire tidy test

help:
	@echo "FileDock Makefile Commands"
	@echo ""
	@echo "前端:"
	@echo "  make deps             安装前端依赖"
	@echo "  make dev              启动前端开发服务"
	@echo "  make web              构建前端并同步 static/dist"
	@echo ""
	@echo "运行与构建:"
	@echo "  make run              启动 Go 服务"
	@echo "  make build            构建当前平台二进制"
	@echo "  make build-lin        构建 Linux AMD64 二进制"
	@echo "  make build-lin-arm    构建 Linux ARM64 二进制"
	@echo "  make air              使用 Air 热更新"
	@echo ""
	@echo "开发工具:"
	@echo "  make wire             生成 Wire 文件"
	@echo "  make tidy             整理 Go 依赖"
	@echo "  make test             运行 Go 测试"
	@echo "  make clean            清理构建产物"

clean:
	@rm -rf $(BIN_DIR) $(FRONTEND_DIR)/dist $(STATIC_DIST_DIR)
	@mkdir -p $(STATIC_DIST_DIR)
	@touch $(STATIC_DIST_DIR)/.placeholder

deps:
	@cd $(FRONTEND_DIR) && bun install

dev:
	@cd $(FRONTEND_DIR) && bun run dev

web:
	@cd $(FRONTEND_DIR) && bun run build
	@rm -rf $(STATIC_DIST_DIR)
	@cp -R $(FRONTEND_DIR)/dist $(STATIC_DIST_DIR)

run:
	@go run . -config $(CONFIG)

build: web
	@mkdir -p $(BIN_DIR)
	@CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o ./$(BIN_DIR)/$(BIN) .

build-lin: web
	@mkdir -p $(BIN_DIR)
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o ./$(BIN_DIR)/$(BIN_LIN) .

build-lin-arm: web
	@mkdir -p $(BIN_DIR)
	@CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o ./$(BIN_DIR)/$(BIN_LIN_ARM) .

air:
	@air -c .air.toml

wire:
	@wire ./wire

tidy:
	@go mod tidy

test:
	@go test ./...
