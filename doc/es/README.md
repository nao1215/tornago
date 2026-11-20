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

[English](../../README.md) | [日本語](../ja/README.md) | [Français](../fr/README.md) | [한국어](../ko/README.md) | [Русский](../ru/README.md) | [中文](../zh-cn/README.md)

# tornago

<img src="../images/tornago-logo-small.png" alt="tornago-logo" width="500"/>

Tornago es un wrapper ligero alrededor de la herramienta de línea de comandos [Tor](https://www.torproject.org/), que proporciona tres funcionalidades principales:

- **Gestión del Daemon Tor**: Iniciar y gestionar procesos Tor programáticamente
- **Cliente Tor**: Enrutar tráfico HTTP/TCP a través del proxy SOCKS5 de Tor con reintentos automáticos
- **Servidor Tor**: Crear y gestionar Servicios Ocultos (servicios onion) a través del ControlPort de Tor

La biblioteca está diseñada tanto para desarrollo (lanzamiento de instancias efímeras de Tor) como para producción (conexión a despliegues de Tor existentes). Probado con éxito en Linux, macOS, Windows y las principales variantes BSD.

## ¿Por qué tornago?

Creé tornago después de conocer la necesidad de rastreo de la web oscura en contextos de prevención de fraude con tarjetas de crédito -- pertenezco al equipo antifraude. Aunque Python se usa comúnmente para rastreo basado en Tor, prefiero Go por su estabilidad y robustez en entornos de producción, así que quería una biblioteca Go para este propósito.

Para prevenir posibles abusos, tornago se mantiene intencionalmente como un wrapper delgado alrededor de la herramienta original de línea de comandos de Tor. He limitado deliberadamente sus características de conveniencia para minimizar el riesgo de abuso.

> [!IMPORTANT]
> **Aviso Legal**: Esta biblioteca está destinada únicamente a propósitos legítimos, como protección de privacidad, investigación de seguridad y actividades autorizadas de prevención de fraude. Los usuarios son los únicos responsables de garantizar que su uso de Tor y esta biblioteca cumple con todas las leyes y regulaciones aplicables. No use esta herramienta para ninguna actividad ilegal.

## Características

- Cero dependencias externas de Go. Construido solo sobre la biblioteca estándar
- Interfaces compatibles con `net.Listener`, `net.Addr`, `net.Dialer` para fácil integración
- Patrón de opciones funcionales para configuración
- Errores estructurados con soporte para `errors.Is`/`errors.As`
- Reintento automático con retroceso exponencial
- Recopilación opcional de métricas y limitación de tasa
- Solo requiere el binario Tor como dependencia externa

## Cómo funciona Tor

Tor (The Onion Router) proporciona anonimato enrutando el tráfico a través de múltiples capas cifradas. Comprender este mecanismo le ayuda a usar tornago de manera efectiva.

El tráfico pasa por tres relés (guardia, intermedio y salida) y cada uno quita una capa de cifrado, así que ningún relé conoce a la vez su IP y su destino. El relé de salida puede leer el tráfico que no está cifrado de extremo a extremo, por lo que debe usar HTTPS. Los tres saltos hacen que las peticiones sean más lentas que una conexión directa. Más detalles en la [documentación del Tor Project](https://support.torproject.org/).

### El Rol de Tornago

Tornago simplifica la integración de Tor manejando:

1. **Comunicación con Proxy SOCKS5**: Enruta automáticamente su tráfico HTTP/TCP a través del proxy SOCKS5 de Tor
2. **Gestión de Circuitos**: Usa ControlPort para rotar circuitos (obtener nuevos nodos de salida)
3. **Creación de Servicios Ocultos**: Gestiona direcciones .onion a través de comandos ADD_ONION/DEL_ONION

## Requisitos

### Go

- **Versión de Go**: 1.25 o posterior

### Sistemas Operativos (Probados en GitHub Actions)

- Linux
- macOS
- Windows
- FreeBSD
- OpenBSD
- NetBSD
- DragonFly BSD

### Tor

Tornago requiere que el daemon Tor esté instalado en su sistema. La biblioteca ha sido probada con Tor versión 0.4.8.x y debería funcionar con versiones más nuevas.

**Instalación:**

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

## Inicio Rápido

Ver [README en inglés](../../README.md#quick-start) para ejemplos de código completos.

## Evasión de Relés Lentos

Tornago incluye un sistema de seguimiento de rendimiento automático que detecta y evita relés Tor lentos. Simplemente habilítelo con una opción y el cliente maneja todo internamente.

### Uso Básico (Recomendado)

```go
// Crear cliente con evasión de relés lentos habilitada
client, err := tornago.NewClient(
    tornago.WithClientSocksAddr(torProcess.SocksAddr()),
    tornago.WithClientControlAddr(torProcess.ControlAddr()),
    tornago.WithSlowRelayAvoidance(),  // Habilitar con valores predeterminados
)
if err != nil {
    log.Fatal(err)
}
defer client.Close()

// Hacer solicitudes normalmente - todo se maneja automáticamente
resp, err := client.Do(req)

// Opcionalmente verificar estadísticas de rendimiento
stats, ok := client.RelayPerformanceStats()
if ok {
    fmt.Printf("Rastreados: %d, Bloqueados: %d\n", stats.TrackedRelays(), stats.BlockedRelays())
}
```

### Configuración de Umbrales Personalizados

```go
// Habilitar con configuración personalizada para requisitos más estrictos
client, err := tornago.NewClient(
    tornago.WithClientSocksAddr(torProcess.SocksAddr()),
    tornago.WithClientControlAddr(torProcess.ControlAddr()),
    tornago.WithSlowRelayAvoidance(
        tornago.SlowRelayMaxLatency(3*time.Second),   // Bloquear relés más lentos que 3s
        tornago.SlowRelayMinSuccessRate(0.9),         // Requerir 90% de tasa de éxito
        tornago.SlowRelayBlockDuration(1*time.Hour),  // Bloquear por 1 hora
        tornago.SlowRelayMinSamples(5),               // Necesitar 5 muestras antes de juzgar
        tornago.SlowRelayMonitorInterval(15*time.Second), // Verificar cada 15s
    ),
)
```

### Valores de Umbral Predeterminados

| Parámetro | Predeterminado | Descripción |
|-----------|---------|-------------|
| MaxLatency | 5 segundos | Relés más lentos se consideran "lentos" |
| MinSuccessRate | 80% | Relés con menor tasa de éxito se bloquean |
| BlockDuration | 30 minutos | Cuánto tiempo permanecen bloqueados los relés lentos |
| MinSamples | 3 | Mediciones mínimas necesarias antes de la evaluación |
| MonitorInterval | 30 segundos | Intervalo de verificación en segundo plano para rotación de circuito |
| AutoExclude | true | Actualizar automáticamente ExcludeNodes de Tor |

Ver [`examples/slow_relay_avoidance`](../../examples/slow_relay_avoidance/main.go) para un ejemplo completo funcional.

## Más Ejemplos

El directorio `examples/` contiene ejemplos funcionales adicionales. Todos los ejemplos están probados y listos para ejecutar.

## Contribuir
¡Las contribuciones son bienvenidas! Consulte la [Guía de Contribución](../../CONTRIBUTING.md) para más detalles.

## Licencia

[Licencia MIT](../../LICENSE)

## Bibliotecas Alternativas, Referencias Oficiales

- [cretz/bine](https://github.com/cretz/bine): Biblioteca Go para acceder e incrustar clientes y servidores Tor
- [wybiral/torgo](https://github.com/wybiral/torgo): Biblioteca Go para interactuar con Tor a través de la interfaz de controlador estándar
- [torproject/stem](https://github.com/torproject/stem): Biblioteca de controlador Python para Tor
- [Wiki Oficial de Tor](https://gitlab.torproject.org/tpo/team/-/wikis/home)
