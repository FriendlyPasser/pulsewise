<p align="center">
  <img src="build/icon/pulsewise.png" width="128" height="128" alt="Pulsewise">
</p>

<h1 align="center">Pulsewise</h1>

<p align="center">把 Mac 的 CPU、GPU 和内存放进菜单栏，一眼看清，一点即开。</p>

<p align="center">
  <img src="https://img.shields.io/badge/platform-macOS-lightgrey" alt="macOS">
  <img src="https://img.shields.io/badge/Go-1.27-00ADD8" alt="Go 1.27">
  <img src="https://img.shields.io/badge/license-MIT-green" alt="MIT">
</p>

<p align="center"><b>简体中文</b> · <a href="README.en.md">English</a></p>

## 特性

- **菜单栏三柱图标**：三根柱子分别对应 CPU、GPU、内存，每秒刷新；模板图标，自动适配深色与浅色菜单栏。悬停显示具体百分比。
- **下拉面板**：左键点击图标展开，CPU 和 GPU 带最近 17 秒的走势曲线。
  - CPU：用户 / 系统 / 空闲占比
  - GPU：使用率、型号、核心数
  - 内存：按“活动监视器”的口径拆分为 App、联动、压缩，另列交换
- **不打扰**：没有 Dock 图标，不进 ⌘Tab；面板不抢焦点，失焦或按 Esc 自动收起，在任意桌面和全屏应用上都能打开。
- **轻量、免权限**：直接调用系统接口采样，无需 root，也没有后台服务。

右键图标可退出。

## 安装与运行

需要 macOS、Go 1.27+ 以及 Xcode 命令行工具（采样代码使用 cgo）。

```sh
git clone https://github.com/FriendlyPasser/pulsewise.git
cd pulsewise
go build -o bin/pulsewise .
./bin/pulsewise
```

调试面板页面时，可以让它在浏览器里运行，不启动菜单栏：

```sh
./bin/pulsewise -web 127.0.0.1:8765
```

## 工作原理

采用与 [magpie](https://github.com/yetone/magpie) 相同的结构：Go + [Wails v3](https://v3alpha.wails.io/) 负责菜单栏图标和面板窗口；面板是一个不依赖框架、不需要打包的小网页，通过同一个 HTTP handler 上的 JSON API 读取 Go 端的数据。

| 路径 | 作用 |
| --- | --- |
| `internal/stats` | 每秒通过 cgo 采样一次：Mach CPU ticks、`host_statistics64` 内存页、sysctl 交换区、IOKit `IOAccelerator` GPU 使用率。 |
| `internal/gui/app.go` | 菜单栏图标（每次采样后重绘）与面板窗口。 |
| `internal/gui/api.go` | 提供页面及接口：`GET /api/state`、`POST /api/fit`、`POST /api/quit`。 |
| `internal/gui/trayicon.go` | 绘制 44px 的菜单栏模板 PNG。 |
| `internal/gui/assets` | 面板页面（HTML / CSS / JS）。 |
| `build/icon` | 应用图标（彩色版及两个单色版）。 |

## 许可证

[MIT](LICENSE)
