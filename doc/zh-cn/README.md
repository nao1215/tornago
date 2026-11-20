<!-- ALL-CONTRIBUTORS-BADGE:START - Do not remove or modify this section -->
[![All Contributors](https://img.shields.io/badge/all_contributors-1-orange.svg?style=flat-square)](../../README.md#contributors-)
<!-- ALL-CONTRIBUTORS-BADGE:END -->
[![MultiPlatformUnitTest](https://github.com/nao1215/tornago/actions/workflows/unit_test.yml/badge.svg)](https://github.com/nao1215/tornago/actions/workflows/unit_test.yml)
![Coverage](https://raw.githubusercontent.com/nao1215/octocovs-central-repo/main/badges/nao1215/tornago/coverage.svg)
[![Gitleaks](https://github.com/nao1215/tornago/actions/workflows/gitleaks.yml/badge.svg)](https://github.com/nao1215/tornago/actions/workflows/gitleaks.yml)
[![Trivy](https://github.com/nao1215/tornago/actions/workflows/trivy.yml/badge.svg)](https://github.com/nao1215/tornago/actions/workflows/trivy.yml)
[![govulncheck](https://github.com/nao1215/tornago/actions/workflows/govulncheck.yml/badge.svg)](https://github.com/nao1215/tornago/actions/workflows/govulncheck.yml)
[![Gosec](https://github.com/nao1215/tornago/actions/workflows/gosec.yml/badge.svg)](https://github.com/nao1215/tornago/actions/workflows/gosec.yml)
[![reviewdog](https://github.com/nao1215/tornago/actions/workflows/reviewdog.yml/badge.svg)](https://github.com/nao1215/tornago/actions/workflows/reviewdog.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/nao1215/tornago.svg)](https://pkg.go.dev/github.com/nao1215/tornago)
[![License](https://img.shields.io/github/license/nao1215/tornago)](../../LICENSE)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/nao1215/tornago/badge)](https://scorecard.dev/viewer/?uri=github.com/nao1215/tornago)

[English](../../README.md) | [日本語](../ja/README.md) | [Español](../es/README.md) | [Français](../fr/README.md) | [한국어](../ko/README.md) | [Русский](../ru/README.md)

# tornago

<img src="../images/tornago-logo-small.png" alt="tornago-logo" width="500"/>

Tornago 是 [Tor](https://www.torproject.org/) 命令行工具的轻量级包装器，提供三个核心功能：

- **Tor 守护进程管理**：以编程方式启动和管理 Tor 进程
- **Tor 客户端**：通过 Tor 的 SOCKS5 代理路由 HTTP/TCP 流量，并自动重试
- **Tor 服务器**：通过 Tor 的 ControlPort 创建和管理隐藏服务（onion 服务）

该库既适用于开发环境（启动临时 Tor 实例）也适用于生产环境（连接到现有 Tor 部署）。已在 Linux、macOS、Windows 和主要 BSD 变体上成功测试。

## 为什么创建 tornago？

在了解到信用卡欺诈防范场景中需要暗网爬虫后，我创建了 tornago（我属于反欺诈团队）。虽然 Python 通常用于基于 Tor 的爬虫，但我更喜欢 Go 在生产环境中的稳定性和健壮性，因此需要一个用于此目的的 Go 库。

为了防止潜在滥用，tornago 有意保持为原始 Tor 命令行工具的薄包装器。我故意限制了其便利功能，以最大程度降低滥用风险。

> [!IMPORTANT]
> **法律声明**：本库仅用于合法目的，如隐私保护、安全研究和经授权的欺诈防范活动。用户有责任确保其使用 Tor 和本库符合所有适用的法律法规。请勿将此工具用于任何非法活动。

## 功能

- Go 零外部依赖。仅基于标准库构建
- 兼容 `net.Listener`、`net.Addr`、`net.Dialer` 接口，易于集成
- 使用函数式选项模式进行配置
- 支持 `errors.Is`/`errors.As` 的结构化错误
- 带指数退避的自动重试
- 可选的指标收集和速率限制
- 仅需 Tor 二进制文件作为外部依赖

## Tor 工作原理

Tor（洋葱路由器）通过多层加密路由流量来提供匿名性。理解此机制有助于有效使用 tornago。

流量依次经过守卫、中间和出口三个中继，每个中继剥掉一层加密，因此没有任何一个中继同时知道你的 IP 和目标地址。出口中继可以读取未做端到端加密的流量，所以请使用 HTTPS。三跳路由也使请求比直连更慢。详情见 [Tor Project 文档](https://support.torproject.org/)。

### Tornago 的角色

Tornago 通过处理以下内容简化 Tor 集成：

1. **SOCKS5 代理通信**：自动通过 Tor 的 SOCKS5 代理路由 HTTP/TCP 流量
2. **电路管理**：使用 ControlPort 轮换电路（获取新的出口节点）
3. **隐藏服务创建**：通过 ADD_ONION/DEL_ONION 命令管理 .onion 地址

## 要求

### Go

- **Go 版本**：1.25 或更高

### 操作系统（已在 GitHub Actions 中测试）

- Linux
- macOS
- Windows
- FreeBSD
- OpenBSD
- NetBSD
- DragonFly BSD

### Tor

Tornago 需要在系统上安装 Tor 守护进程。该库已使用 Tor 版本 0.4.8.x 进行测试，应该适用于更新版本。

**安装：**

```bash
# Ubuntu/Debian
sudo apt update
sudo apt install tor

# Fedora/RHEL
sudo dnf install tor

# Arch Linux
sudo pacman -S tor

# macOS (Homebrew)
brew install tor
```

## 快速入门

完整代码示例请参见[英文 README](../../README.md#quick-start)。

## 慢速中继规避

Tornago 包含一个自动性能跟踪系统，可检测并规避慢速 Tor 中继。只需通过一个选项启用，客户端会在内部处理一切。

### 基本用法（推荐）

```go
// 创建启用慢速中继规避的客户端
client, err := tornago.NewClient(
    tornago.WithClientSocksAddr(torProcess.SocksAddr()),
    tornago.WithClientControlAddr(torProcess.ControlAddr()),
    tornago.WithSlowRelayAvoidance(),  // 使用默认设置启用
)
if err != nil {
    log.Fatal(err)
}
defer client.Close()

// 正常发送请求 - 一切自动处理
resp, err := client.Do(req)

// 可选地检查性能统计
stats, ok := client.RelayPerformanceStats()
if ok {
    fmt.Printf("跟踪中: %d, 已阻止: %d\n", stats.TrackedRelays(), stats.BlockedRelays())
}
```

### 自定义阈值配置

```go
// 使用自定义设置启用，满足更严格的要求
client, err := tornago.NewClient(
    tornago.WithClientSocksAddr(torProcess.SocksAddr()),
    tornago.WithClientControlAddr(torProcess.ControlAddr()),
    tornago.WithSlowRelayAvoidance(
        tornago.SlowRelayMaxLatency(3*time.Second),   // 阻止慢于3秒的中继
        tornago.SlowRelayMinSuccessRate(0.9),         // 要求90%成功率
        tornago.SlowRelayBlockDuration(1*time.Hour),  // 阻止1小时
        tornago.SlowRelayMinSamples(5),               // 判断前需要5个样本
        tornago.SlowRelayMonitorInterval(15*time.Second), // 每15秒检查一次
    ),
)
```

### 默认阈值

| 参数 | 默认值 | 描述 |
|-----------|---------|-------------|
| MaxLatency | 5秒 | 比这慢的中继被视为"慢速" |
| MinSuccessRate | 80% | 成功率较低的中继会被阻止 |
| BlockDuration | 30分钟 | 慢速中继保持阻止的时长 |
| MinSamples | 3 | 评估前所需的最小测量次数 |
| MonitorInterval | 30秒 | 电路轮换的后台检查间隔 |
| AutoExclude | true | 自动更新 Tor 的 ExcludeNodes |

完整工作示例请参见 [`examples/slow_relay_avoidance`](../../examples/slow_relay_avoidance/main.go)。

## 更多示例

`examples/` 目录包含其他工作示例。所有示例均已测试并可立即运行。

## 贡献

欢迎贡献！详情请参见[贡献指南](../../CONTRIBUTING.md)。

## 许可证

[MIT 许可证](../../LICENSE)

## 替代库、官方参考

- [cretz/bine](https://github.com/cretz/bine)：用于访问和嵌入 Tor 客户端和服务器的 Go 库
- [wybiral/torgo](https://github.com/wybiral/torgo)：通过标准控制器接口与 Tor 交互的 Go 库
- [torproject/stem](https://github.com/torproject/stem)：Tor 的 Python 控制器库
- [Tor 官方 Wiki](https://gitlab.torproject.org/tpo/team/-/wikis/home)
