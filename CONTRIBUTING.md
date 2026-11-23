# 参与开发

感谢你愿意帮一把。这个项目的界面与播放都在 `internal/` 里，读起来不算难；下面几件事照做，改动会顺利很多。

## 先把项目跑起来

```bash
# 需要 Go 1.27+
go tool mygo dev        # 开发模式运行
go tool mygo build      # 打包到 build/
go test ./internal/...  # 跑测试（约十几分钟：界面测试要真的画帧）
```

想用汽水 / 酷狗 / QQ 音乐这三家，另需 Node.js（≥ 20）：它们的登录与取流由 `sidecar/` 里的 Node 进程完成。

## 提交之前

- `gofmt -w` 改过的文件（CI 的 `go vet` 会拦下明显问题）
- **跑一遍测试**：`go test ./internal/...`。改了界面就多跑几次——界面测试会真的画帧，偶发失败说明有竞态，不是"再跑一次就好"
- 提交信息用中文，第一行说清"做了什么"，正文说清"为什么"（对照 `git log` 里已有的写法）
- 一个提交做一件事：修 bug 与加功能分开，别混在一起

## 改动落在哪里

| 想改的东西 | 去哪 |
|---|---|
| 页面、组件、动效驱动 | `internal/app/` |
| GPU 上的特效与配色 | `internal/fx/` |
| 解码、队列、均衡器、切歌过渡 | `internal/player/` |
| 音源接入（网易云 / 汽水 / 酷狗 / QQ） | `internal/{netease,qishui,kugou,qq}/` 与 `sidecar/` |
| 官网 | `website/`（`bun run dev` 起本地） |

## 提 PR

- 从 `main` 拉分支，PR 对着 `main` 提
- 描述里写：改了什么、为什么、怎么验证的（截图/命令/测试名都行）
- 如果改了界面，附一张截图或一段录屏，审起来快很多

## 报告问题

用 issue 模板：提 bug 时带上系统版本、应用版本、复现步骤与日志（设置 → 报告问题会帮你打包）。

安全相关的问题请不要开公开 issue，见 [SECURITY.md](SECURITY.md)。