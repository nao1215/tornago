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

[English](../../README.md) | [日本語](../ja/README.md) | [Español](../es/README.md) | [Français](../fr/README.md) | [Русский](../ru/README.md) | [中文](../zh-cn/README.md)

# tornago

<img src="../images/tornago-logo-small.png" alt="tornago-logo" width="500"/>

Tornago는 [Tor](https://www.torproject.org/) 명령줄 도구를 감싸는 경량 래퍼로, 세 가지 핵심 기능을 제공합니다:

- **Tor 데몬 관리**: Tor 프로세스를 프로그래밍 방식으로 시작하고 관리
- **Tor 클라이언트**: 자동 재시도 기능을 갖춘 Tor의 SOCKS5 프록시를 통한 HTTP/TCP 트래픽 라우팅
- **Tor 서버**: Tor의 ControlPort를 통한 히든 서비스(onion 서비스) 생성 및 관리

이 라이브러리는 개발 환경(임시 Tor 인스턴스 시작)과 프로덕션 환경(기존 Tor 배포에 연결) 모두를 위해 설계되었습니다. Linux, macOS, Windows 및 주요 BSD 계열에서 성공적으로 테스트되었습니다.

## 왜 tornago를 만들었나요?

신용카드 사기 방지 맥락에서 다크웹 크롤링의 필요성을 알게 된 후 tornago를 만들었습니다(저는 사기 방지 팀에 속해 있습니다). Tor 기반 크롤링에는 일반적으로 Python이 사용되지만, 프로덕션 환경에서의 안정성과 견고성 때문에 Go를 선호하므로 이 목적을 위한 Go 라이브러리가 필요했습니다.

잠재적인 악용을 방지하기 위해 tornago는 의도적으로 원본 Tor 명령줄 도구의 얇은 래퍼로 유지됩니다. 악용 위험을 최소화하기 위해 편의 기능을 의도적으로 제한했습니다.

> [!IMPORTANT]
> **법적 고지**: 이 라이브러리는 개인정보 보호, 보안 연구 및 승인된 사기 방지 활동과 같은 합법적인 목적으로만 사용됩니다. 사용자는 Tor 및 이 라이브러리의 사용이 모든 관련 법률 및 규정을 준수하는지 확인할 책임이 있습니다. 불법 활동에 이 도구를 사용하지 마십시오.

## 기능

- Go 외부 종속성 없음. 표준 라이브러리만으로 구축
- 쉬운 통합을 위한 `net.Listener`, `net.Addr`, `net.Dialer` 호환 인터페이스
- 구성을 위한 함수형 옵션 패턴
- `errors.Is`/`errors.As` 지원을 갖춘 구조화된 오류
- 지수 백오프를 사용한 자동 재시도
- 선택적 메트릭 수집 및 속도 제한
- 외부 종속성으로 Tor 바이너리만 필요

## Tor 작동 방식

Tor(The Onion Router)는 여러 암호화 계층을 통해 트래픽을 라우팅하여 익명성을 제공합니다. 이 메커니즘을 이해하면 tornago를 효과적으로 사용하는 데 도움이 됩니다.

트래픽은 가드, 중간, 출구의 세 릴레이를 거치며 각 릴레이가 암호화를 한 겹씩 벗기므로, 사용자의 IP와 목적지를 모두 아는 릴레이는 없습니다. 출구 릴레이는 종단 간 암호화되지 않은 트래픽을 읽을 수 있으니 HTTPS를 사용하세요. 세 번의 홉 때문에 직접 연결보다 느립니다. 자세한 내용은 [Tor Project 문서](https://support.torproject.org/)를 참고하세요.

### Tornago의 역할

Tornago는 다음을 처리하여 Tor 통합을 단순화합니다:

1. **SOCKS5 프록시 통신**: HTTP/TCP 트래픽을 Tor의 SOCKS5 프록시를 통해 자동으로 라우팅
2. **회로 관리**: ControlPort를 사용하여 회로 순환 (새로운 출구 노드 얻기)
3. **히든 서비스 생성**: ADD_ONION/DEL_ONION 명령을 통한 .onion 주소 관리

## 요구사항

### Go

- **Go 버전**: 1.25 이상

### 운영체제 (GitHub Actions에서 테스트됨)

- Linux
- macOS
- Windows
- FreeBSD
- OpenBSD
- NetBSD
- DragonFly BSD

### Tor

Tornago는 시스템에 Tor 데몬이 설치되어 있어야 합니다. 라이브러리는 Tor 버전 0.4.8.x로 테스트되었으며 최신 버전에서도 작동합니다.

**설치:**

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

## 빠른 시작

전체 코드 예제는 [영문 README](../../README.md#quick-start)를 참조하세요.

## 느린 릴레이 회피

Tornago에는 느린 Tor 릴레이를 자동으로 감지하고 회피하는 성능 추적 시스템이 포함되어 있습니다. 하나의 옵션으로 활성화하기만 하면 클라이언트가 내부적으로 모든 것을 처리합니다.

### 기본 사용법 (권장)

```go
// 느린 릴레이 회피가 활성화된 클라이언트 생성
client, err := tornago.NewClient(
    tornago.WithClientSocksAddr(torProcess.SocksAddr()),
    tornago.WithClientControlAddr(torProcess.ControlAddr()),
    tornago.WithSlowRelayAvoidance(),  // 기본값으로 활성화
)
if err != nil {
    log.Fatal(err)
}
defer client.Close()

// 정상적으로 요청 실행 - 모든 것이 자동으로 처리됩니다
resp, err := client.Do(req)

// 선택적으로 성능 통계 확인
stats, ok := client.RelayPerformanceStats()
if ok {
    fmt.Printf("추적 중: %d, 차단됨: %d\n", stats.TrackedRelays(), stats.BlockedRelays())
}
```

### 사용자 정의 임계값 설정

```go
// 더 엄격한 요구 사항을 위한 사용자 정의 설정으로 활성화
client, err := tornago.NewClient(
    tornago.WithClientSocksAddr(torProcess.SocksAddr()),
    tornago.WithClientControlAddr(torProcess.ControlAddr()),
    tornago.WithSlowRelayAvoidance(
        tornago.SlowRelayMaxLatency(3*time.Second),   // 3초보다 느린 릴레이 차단
        tornago.SlowRelayMinSuccessRate(0.9),         // 90% 성공률 요구
        tornago.SlowRelayBlockDuration(1*time.Hour),  // 1시간 동안 차단
        tornago.SlowRelayMinSamples(5),               // 판단 전 5개 샘플 필요
        tornago.SlowRelayMonitorInterval(15*time.Second), // 15초마다 확인
    ),
)
```

### 기본 임계값

| 매개변수 | 기본값 | 설명 |
|-----------|---------|-------------|
| MaxLatency | 5초 | 이보다 느린 릴레이는 "느림"으로 간주됩니다 |
| MinSuccessRate | 80% | 성공률이 낮은 릴레이는 차단됩니다 |
| BlockDuration | 30분 | 느린 릴레이가 차단되는 기간 |
| MinSamples | 3 | 평가 전 필요한 최소 측정 횟수 |
| MonitorInterval | 30초 | 회로 교체를 위한 백그라운드 확인 간격 |
| AutoExclude | true | Tor의 ExcludeNodes 자동 업데이트 |

전체 작동 예제는 [`examples/slow_relay_avoidance`](../../examples/slow_relay_avoidance/main.go)를 참조하세요.

## 더 많은 예제

`examples/` 디렉토리에는 추가 작동 예제가 포함되어 있습니다. 모든 예제는 테스트되어 바로 실행할 수 있습니다.

## 기여

기여를 환영합니다! 자세한 내용은 [기여 가이드](../../CONTRIBUTING.md)를 참조하세요.

## 라이선스

[MIT 라이선스](../../LICENSE)

## 대체 라이브러리, 공식 참조

- [cretz/bine](https://github.com/cretz/bine): Tor 클라이언트 및 서버에 액세스하고 임베드하기 위한 Go 라이브러리
- [wybiral/torgo](https://github.com/wybiral/torgo): 표준 컨트롤러 인터페이스를 통해 Tor와 상호작용하는 Go 라이브러리
- [torproject/stem](https://github.com/torproject/stem): Tor용 Python 컨트롤러 라이브러리
- [Tor 공식 Wiki](https://gitlab.torproject.org/tpo/team/-/wikis/home)
