<p align="center">
  <img src="build/icon/pulsewise.png" width="128" height="128" alt="Pulsewise">
</p>

<h1 align="center">Pulsewise</h1>

<p align="center">Your Mac's CPU, GPU and memory in the menu bar: readable at a glance, one click for the details.</p>

<p align="center">
  <img src="https://img.shields.io/badge/platform-macOS-lightgrey" alt="macOS">
  <img src="https://img.shields.io/badge/Go-1.27-00ADD8" alt="Go 1.27">
  <img src="https://img.shields.io/badge/license-MIT-green" alt="MIT">
</p>

<p align="center"><a href="README.md">简体中文</a> · <b>English</b></p>

<p align="center">
  <img src="docs/screenshots/panel-light.png" width="340" alt="The Pulsewise panel in light mode">
  <img src="docs/screenshots/panel-dark.png" width="340" alt="The Pulsewise panel in dark mode">
</p>

## Features

- **Three-bar menu bar icon**: one bar each for CPU, GPU and memory, refreshed every second. It is a template image, so it suits both dark and light menu bars. Hover for exact percentages.
- **Dropdown panel**: left-click the icon to open it. CPU and GPU come with a sparkline of the last 17 seconds.
  - CPU: user / system / idle
  - GPU: utilization, model, core count
  - Memory: split the way Activity Monitor splits it (App, wired, compressed), plus swap
- **Stays out of the way**: no Dock icon, not in ⌘Tab. The panel doesn't take focus, hides when it loses focus or on Esc, and opens on any Space, including over full-screen apps.
- **Light, no permissions**: samples through system APIs directly. No root, no background service.

Right-click the icon to quit.

> The panel's labels are in Chinese.

## Build and run

Requires macOS, Go 1.27+ and the Xcode command line tools (the sampler uses cgo).

```sh
git clone https://github.com/FriendlyPasser/pulsewise.git
cd pulsewise
go build -o bin/pulsewise .
./bin/pulsewise
```

To work on the panel page, serve it to a browser instead of the menu bar:

```sh
./bin/pulsewise -web 127.0.0.1:8765
```

## License

[MIT](LICENSE)
