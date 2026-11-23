<div align="center">

# Echo 云音乐

### A native desktop music client — no WebView

[简体中文](README.md) | English

<br>

![release](https://img.shields.io/github/v/release/Simqiqi/Echo-music?style=for-the-badge&label=release&color=blue)
![downloads](https://img.shields.io/github/downloads/Simqiqi/Echo-music/total?style=for-the-badge&color=green)
![license](https://img.shields.io/badge/license-AGPL--3.0-orange?style=for-the-badge)
![stars](https://img.shields.io/github/stars/Simqiqi/Echo-music?style=for-the-badge&color=yellow)

<br>

![go](https://img.shields.io/badge/Go-%3E%3D1.27-00ADD8?style=for-the-badge&logo=go&logoColor=white)
![macOS](https://img.shields.io/badge/macOS-Apple%20Silicon%20%7C%20Intel-black?style=for-the-badge&logo=apple&logoColor=white)
![Windows](https://img.shields.io/badge/Windows-x64%20%7C%20arm64-0078D6?style=for-the-badge)
![Linux](https://img.shields.io/badge/Linux-x64%20%7C%20arm64-FCC624?style=for-the-badge&logo=linux&logoColor=black)

</div>

---

A desktop music client written in **Go** with [MyGo](https://github.com/egoist/mygo) — **a native interface, no WebView**, cold start in under a second. It began as a fork of [mygo-cloudmusic](https://github.com/egoist/mygo-cloudmusic) and has grown its own ideas: liquid glass, a vinyl turntable, per-character lyrics, a weekly listening report, a weather radio — and, since 0.4.2, **four music services in one window**.

The interface is Chinese-only for now; this file is a short introduction for English readers.

## Four services, one window

Sign in to **NetEase Cloud Music**, **Qishui Music**, **Kugou Music** and **QQ Music**. Each account's playlists get their own group in the sidebar, search can run against all four, and a playlist can be carried from one service to another.

| Service | Sign-in | Read (playlists / playback / lyrics) | Write (add songs to a playlist) |
|---|---|---|---|
| **NetEase Cloud Music** | QR code | ✅ | ✅ create and fill a playlist |
| **Qishui Music** | QR code | ✅ | ✅ into an existing playlist |
| **Kugou Music** | QR code | ✅ | ❌ read-only third-party API |
| **QQ Music** | **QQ or WeChat QR code** | ✅ | ❌ read-only third-party API |

> Qishui, Kugou and QQ Music sign their requests in ways that cannot be reproduced in Go, so a small **Node sidecar** bundled with the app does their sign-in and streaming — that means **Node.js (≥ 20) is needed to use those three**. NetEase alone needs nothing extra.

## Install

Download from [Releases](https://github.com/Simqiqi/Echo-music/releases/latest), or install with one line (into your home directory; it updates itself afterwards):

**macOS / Linux**

```bash
curl -fsSL https://simqiqiecho.dpdns.org/install.sh | sh
```

**Windows (PowerShell)**

```powershell
irm https://simqiqiecho.dpdns.org/install.ps1 | iex
```

> **macOS says the app "is damaged"?** That is Gatekeeper's quarantine flag on an unsigned download, not a broken file:
>
> ```bash
> xattr -dr com.apple.quarantine /Applications/Echo云音乐.app
> ```

## Build from source

Go 1.27+ (plus Node.js if you want the sidecar-backed services):

```bash
go tool mygo dev     # run in development
go tool mygo build   # package into build/
go test ./internal/...   # tests
```

## Licence

[AGPL-3.0](LICENSE). Not affiliated with any of the music services; all music rights belong to their owners.
