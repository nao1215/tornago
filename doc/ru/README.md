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

[English](../../README.md) | [日本語](../ja/README.md) | [Español](../es/README.md) | [Français](../fr/README.md) | [한국어](../ko/README.md) | [中文](../zh-cn/README.md)

# tornago

<img src="../images/tornago-logo-small.png" alt="tornago-logo" width="500"/>

Tornago — это легкая обёртка вокруг инструмента командной строки [Tor](https://www.torproject.org/), предоставляющая три основные функции:

- **Управление демоном Tor**: Запуск и управление процессами Tor программно
- **Клиент Tor**: Маршрутизация HTTP/TCP трафика через прокси SOCKS5 Tor с автоматическими повторами
- **Сервер Tor**: Создание и управление скрытыми сервисами (onion-сервисы) через ControlPort Tor

Библиотека разработана как для разработки (запуск временных экземпляров Tor), так и для продакшена (подключение к существующим развёрнутым Tor). Успешно протестирована в Linux, macOS, Windows и основных вариантах BSD.

## Почему tornago?

Я создал tornago после того, как узнал о необходимости сканирования даркнета в контексте предотвращения мошенничества с кредитными картами -- я работаю в команде по борьбе с мошенничеством. Хотя Python обычно используется для сканирования на основе Tor, я предпочитаю Go за его стабильность и надёжность в производственных средах, поэтому мне нужна была библиотека Go для этой цели.

Чтобы предотвратить возможное злоупотребление, tornago намеренно оставлен в виде тонкой обёртки вокруг оригинального инструмента командной строки Tor. Я намеренно ограничил его удобные функции, чтобы минимизировать риск злоупотребления.

> [!IMPORTANT]
> **Юридическое уведомление**: Эта библиотека предназначена только для законных целей, таких как защита конфиденциальности, исследования безопасности и разрешённая деятельность по предотвращению мошенничества. Пользователи несут единоличную ответственность за обеспечение соответствия их использования Tor и этой библиотеки всем применимым законам и нормативным актам. Не используйте этот инструмент для каких-либо незаконных действий.

## Возможности

- Нулевые внешние зависимости Go. Построено только на стандартной библиотеке
- Интерфейсы, совместимые с `net.Listener`, `net.Addr`, `net.Dialer` для легкой интеграции
- Паттерн функциональных опций для конфигурации
- Структурированные ошибки с поддержкой `errors.Is`/`errors.As`
- Автоматический повтор с экспоненциальной задержкой
- Опциональный сбор метрик и ограничение скорости
- Требуется только бинарный файл Tor в качестве внешней зависимости

## Как работает Tor

Tor (The Onion Router) обеспечивает анонимность, маршрутизируя трафик через несколько зашифрованных слоёв. Понимание этого механизма помогает эффективно использовать tornago.

Трафик проходит через три реле (сторожевое, промежуточное и выходное), и каждое снимает один слой шифрования, поэтому ни одно реле не знает одновременно ваш IP и адрес назначения. Выходное реле может читать трафик, не зашифрованный из конца в конец, поэтому используйте HTTPS. Из-за трёх переходов запросы медленнее, чем при прямом соединении. Подробнее в [документации Tor Project](https://support.torproject.org/).

### Роль Tornago

Tornago упрощает интеграцию Tor, обрабатывая:

1. **Связь с прокси SOCKS5**: Автоматически маршрутизирует HTTP/TCP трафик через прокси SOCKS5 Tor
2. **Управление цепями**: Использует ControlPort для ротации цепей (получение новых выходных узлов)
3. **Создание скрытых сервисов**: Управляет адресами .onion через команды ADD_ONION/DEL_ONION

## Требования

### Go

- **Версия Go**: 1.25 или новее

### Операционные системы (протестировано в GitHub Actions)

- Linux
- macOS
- Windows
- FreeBSD
- OpenBSD
- NetBSD
- DragonFly BSD

### Tor

Tornago требует установки демона Tor в вашей системе. Библиотека протестирована с Tor версии 0.4.8.x и должна работать с новыми версиями.

**Установка:**

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

## Быстрый старт

См. [README на английском](../../README.md#quick-start) для полных примеров кода.

## Обход медленных реле

Tornago включает автоматическую систему отслеживания производительности, которая обнаруживает и избегает медленные Tor-реле. Просто включите её одной опцией, и клиент будет обрабатывать всё внутренне.

### Базовое использование (рекомендуется)

```go
// Create client with slow relay avoidance enabled
clientCfg, err := tornago.NewClientConfig(
	tornago.WithClientSocksAddr(torProcess.SocksAddr()),
	tornago.WithClientControlAddr(torProcess.ControlAddr()),
	tornago.WithSlowRelayAvoidance(), // Enable with defaults
)
if err != nil {
	log.Fatal(err)
}

client, err := tornago.NewClient(clientCfg)
if err != nil {
	log.Fatal(err)
}
defer client.Close()

// Make requests normally; everything is handled automatically
resp, err := client.Do(req)
if err != nil {
	log.Fatal(err)
}
defer resp.Body.Close()

// Optionally check performance statistics
stats, ok := client.RelayPerformanceStats()
if ok {
	fmt.Printf("Tracked: %d, Blocked: %d\n", stats.TrackedRelays(), stats.BlockedRelays())
}
```

### Настройка пользовательских порогов

```go
// Enable with custom settings for stricter requirements
clientCfg, err := tornago.NewClientConfig(
	tornago.WithClientSocksAddr(torProcess.SocksAddr()),
	tornago.WithClientControlAddr(torProcess.ControlAddr()),
	tornago.WithSlowRelayAvoidance(
		tornago.SlowRelayMaxLatency(3*time.Second),       // Block relays slower than 3s
		tornago.SlowRelayMinSuccessRate(0.9),             // Require 90% success rate
		tornago.SlowRelayBlockDuration(1*time.Hour),      // Block for 1 hour
		tornago.SlowRelayMinSamples(5),                   // Need 5 samples before judging
		tornago.SlowRelayMonitorInterval(15*time.Second), // Check every 15s
	),
)
if err != nil {
	log.Fatal(err)
}

client, err := tornago.NewClient(clientCfg)
if err != nil {
	log.Fatal(err)
}
defer client.Close()
```

### Пороговые значения по умолчанию

| Параметр | По умолчанию | Описание |
|-----------|---------|-------------|
| MaxLatency | 5 секунд | Реле медленнее считаются "медленными" |
| MinSuccessRate | 80% | Реле с более низким показателем успеха блокируются |
| BlockDuration | 30 минут | Как долго медленные реле остаются заблокированными |
| MinSamples | 3 | Минимум измерений до оценки |
| MonitorInterval | 30 секунд | Интервал фоновой проверки для ротации цепей |
| AutoExclude | true | Автоматически обновлять ExcludeNodes Tor |

См. [`examples/slow_relay_avoidance`](../../examples/slow_relay_avoidance/main.go) для полного рабочего примера.

## Дополнительные примеры

Директория `examples/` содержит дополнительные рабочие примеры. Все примеры протестированы и готовы к запуску.

## Вклад

Вклады приветствуются! Подробности см. в [Руководстве по внесению вклада](../../CONTRIBUTING.md).

## Лицензия

[Лицензия MIT](../../LICENSE)

## Альтернативные библиотеки, официальные справочники

- [cretz/bine](https://github.com/cretz/bine): Библиотека Go для доступа и встраивания клиентов и серверов Tor
- [wybiral/torgo](https://github.com/wybiral/torgo): Библиотека Go для взаимодействия с Tor через стандартный интерфейс контроллера
- [torproject/stem](https://github.com/torproject/stem): Библиотека контроллера Python для Tor
- [Официальная Wiki Tor](https://gitlab.torproject.org/tpo/team/-/wikis/home)
