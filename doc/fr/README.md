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

[English](../../README.md) | [日本語](../ja/README.md) | [Español](../es/README.md) | [한국어](../ko/README.md) | [Русский](../ru/README.md) | [中文](../zh-cn/README.md)

# tornago

<img src="../images/tornago-logo-small.png" alt="tornago-logo" width="500"/>

Tornago est un wrapper léger autour de l'outil en ligne de commande [Tor](https://www.torproject.org/), fournissant trois fonctionnalités principales :

- **Gestion du Daemon Tor** : Lancer et gérer les processus Tor par programme
- **Client Tor** : Acheminer le trafic HTTP/TCP via le proxy SOCKS5 de Tor avec réessais automatiques
- **Serveur Tor** : Créer et gérer des Services Cachés (services onion) via le ControlPort de Tor

La bibliothèque est conçue à la fois pour le développement (lancement d'instances éphémères de Tor) et la production (connexion aux déploiements Tor existants). Testée avec succès sur Linux, macOS, Windows et les principales variantes BSD.

## Pourquoi tornago ?

J'ai créé tornago après avoir appris le besoin d'exploration du dark web dans les contextes de prévention de la fraude par carte de crédit -- j'appartiens à l'équipe anti-fraude. Bien que Python soit couramment utilisé pour l'exploration basée sur Tor, je préfère Go pour sa stabilité et sa robustesse dans les environnements de production, j'avais donc besoin d'une bibliothèque Go pour cet objectif.

Pour prévenir les abus potentiels, tornago est intentionnellement maintenu comme un wrapper léger autour de l'outil en ligne de commande Tor original. J'ai délibérément limité ses fonctionnalités de commodité pour minimiser le risque d'abus.

> [!IMPORTANT]
> **Avis Légal** : Cette bibliothèque est destinée uniquement à des fins légitimes, telles que la protection de la vie privée, la recherche en sécurité et les activités autorisées de prévention de la fraude. Les utilisateurs sont seuls responsables de s'assurer que leur utilisation de Tor et de cette bibliothèque est conforme à toutes les lois et réglementations applicables. N'utilisez pas cet outil pour des activités illégales.

## Fonctionnalités

- Zéro dépendances externes Go. Construit uniquement sur la bibliothèque standard
- Interfaces compatibles `net.Listener`, `net.Addr`, `net.Dialer` pour une intégration facile
- Modèle d'options fonctionnelles pour la configuration
- Erreurs structurées avec support `errors.Is`/`errors.As`
- Réessai automatique avec backoff exponentiel
- Collecte optionnelle de métriques et limitation de taux
- Nécessite uniquement le binaire Tor comme dépendance externe

## Comment fonctionne Tor

Tor (The Onion Router) fournit l'anonymat en acheminant le trafic à travers plusieurs couches chiffrées. Comprendre ce mécanisme vous aide à utiliser tornago efficacement.

Le trafic traverse trois relais (garde, intermédiaire, sortie) et chacun retire une couche de chiffrement, si bien qu'aucun relais ne connaît à la fois votre IP et votre destination. Le relais de sortie peut lire le trafic qui n'est pas chiffré de bout en bout, utilisez donc HTTPS. Les trois sauts rendent les requêtes plus lentes qu'une connexion directe. Plus de détails dans la [documentation du Tor Project](https://support.torproject.org/).

### Le Rôle de Tornago

Tornago simplifie l'intégration Tor en gérant :

1. **Communication Proxy SOCKS5** : Achemine automatiquement votre trafic HTTP/TCP via le proxy SOCKS5 de Tor
2. **Gestion des Circuits** : Utilise ControlPort pour faire tourner les circuits (obtenir de nouveaux nœuds de sortie)
3. **Création de Services Cachés** : Gère les adresses .onion via les commandes ADD_ONION/DEL_ONION

## Exigences

### Go

- **Version Go** : 1.25 ou ultérieure

### Systèmes d'Exploitation (Testés dans GitHub Actions)

- Linux
- macOS
- Windows
- FreeBSD
- OpenBSD
- NetBSD
- DragonFly BSD

### Tor

Tornago nécessite que le daemon Tor soit installé sur votre système. La bibliothèque a été testée avec Tor version 0.4.8.x et devrait fonctionner avec des versions plus récentes.

**Installation :**

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

## Démarrage Rapide

Voir [README en anglais](../../README.md#quick-start) pour des exemples de code complets.

## Évitement des Relais Lents

Tornago inclut un système de suivi des performances automatique qui détecte et évite les relais Tor lents. Activez-le simplement avec une option et le client gère tout en interne.

### Utilisation de Base (Recommandée)

```go
// Créer un client avec l'évitement des relais lents activé
client, err := tornago.NewClient(
    tornago.WithClientSocksAddr(torProcess.SocksAddr()),
    tornago.WithClientControlAddr(torProcess.ControlAddr()),
    tornago.WithSlowRelayAvoidance(),  // Activer avec les valeurs par défaut
)
if err != nil {
    log.Fatal(err)
}
defer client.Close()

// Faire des requêtes normalement - tout est géré automatiquement
resp, err := client.Do(req)

// Optionnellement vérifier les statistiques de performance
stats, ok := client.RelayPerformanceStats()
if ok {
    fmt.Printf("Suivis: %d, Bloqués: %d\n", stats.TrackedRelays(), stats.BlockedRelays())
}
```

### Configuration des Seuils Personnalisés

```go
// Activer avec des paramètres personnalisés pour des exigences plus strictes
client, err := tornago.NewClient(
    tornago.WithClientSocksAddr(torProcess.SocksAddr()),
    tornago.WithClientControlAddr(torProcess.ControlAddr()),
    tornago.WithSlowRelayAvoidance(
        tornago.SlowRelayMaxLatency(3*time.Second),   // Bloquer les relais plus lents que 3s
        tornago.SlowRelayMinSuccessRate(0.9),         // Exiger 90% de taux de réussite
        tornago.SlowRelayBlockDuration(1*time.Hour),  // Bloquer pendant 1 heure
        tornago.SlowRelayMinSamples(5),               // Besoin de 5 échantillons avant jugement
        tornago.SlowRelayMonitorInterval(15*time.Second), // Vérifier toutes les 15s
    ),
)
```

### Valeurs de Seuil par Défaut

| Paramètre | Par défaut | Description |
|-----------|---------|-------------|
| MaxLatency | 5 secondes | Les relais plus lents sont considérés comme "lents" |
| MinSuccessRate | 80% | Les relais avec un taux de réussite plus bas sont bloqués |
| BlockDuration | 30 minutes | Durée de blocage des relais lents |
| MinSamples | 3 | Mesures minimales nécessaires avant l'évaluation |
| MonitorInterval | 30 secondes | Intervalle de vérification en arrière-plan pour la rotation des circuits |
| AutoExclude | true | Mettre à jour automatiquement ExcludeNodes de Tor |

Voir [`examples/slow_relay_avoidance`](../../examples/slow_relay_avoidance/main.go) pour un exemple complet fonctionnel.

## Plus d'Exemples

Le répertoire `examples/` contient des exemples fonctionnels supplémentaires. Tous les exemples sont testés et prêts à exécuter.

## Contribuer
Les contributions sont les bienvenues ! Consultez le [Guide de Contribution](../../CONTRIBUTING.md) pour plus de détails.

## Licence

[Licence MIT](../../LICENSE)

## Bibliothèques Alternatives, Références Officielles

- [cretz/bine](https://github.com/cretz/bine) : Bibliothèque Go pour accéder et intégrer des clients et serveurs Tor
- [wybiral/torgo](https://github.com/wybiral/torgo) : Bibliothèque Go pour interagir avec Tor via l'interface de contrôleur standard
- [torproject/stem](https://github.com/torproject/stem) : Bibliothèque de contrôleur Python pour Tor
- [Wiki Officiel Tor](https://gitlab.torproject.org/tpo/team/-/wikis/home)
