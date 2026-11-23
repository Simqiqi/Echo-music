## 这版改了什么

<!-- 一两句话说明做什么、为什么。 -->

## 怎么验证的

<!-- 测试名、命令、截图、录屏都行。改了界面请附截图。 -->

- [ ] 跑过 `go test ./internal/...`
- [ ] 改过界面：附了截图或录屏
- [ ] 改过 `sidecar/`：确认三家登录/取流仍然正常

## 影响到哪里

<!-- 勾选受影响的模块 -->
- [ ] `internal/app`（界面与交互）
- [ ] `internal/fx`（特效）
- [ ] `internal/player`（播放与均衡器）
- [ ] 音源接入（`internal/{netease,qishui,kugou,qq}`、`sidecar/`）
- [ ] `website`（官网）
- [ ] 文档 / 构建 / CI
