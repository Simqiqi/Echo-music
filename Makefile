# 常用命令收在这里：make dev / make build / make test / make site
# 需要 Go 1.27+；官网那几条另需 bun。

GO ?= go

.PHONY: dev build test vet fmt site site-build site-deploy clean

dev: ## 开发模式运行
	$(GO) tool mygo dev

build: ## 打包到 build/
	$(GO) tool mygo build

test: ## 跑测试（界面测试要真的画帧，十来分钟）
	$(GO) test ./internal/...

vet: ## 静态检查
	$(GO) vet ./...

fmt: ## 格式化所有 Go 文件
	gofmt -w $$(git ls-files '*.go')

site: ## 官网开发模式（localhost:3000）
	bun run dev

site-build: ## 构建官网到 website/dist
	bun run build

site-deploy: ## 部署官网（需要先 wrangler login）
	bun run deploy

clean: ## 删掉打包产物
	rm -rf build
