# Testing

`make test` runs the deterministic tests (`go test -short`), and `make integration-test` adds the tests that launch a Tor daemon and reach the Tor network. CI runs the integration tests on Linux, macOS, Windows and the four BSDs. The long fuzz and property searches run on a schedule.

## Unit and integration tests

Every source file has a `_test.go` beside it. Tests that need Tor call `requireIntegration` or the shared test server in `testserver_helper_test.go`, and skip themselves when `TORNAGO_INTEGRATION=1` is not set, when `tor` is not installed, or when the daemon cannot reach the network. `example_test.go` holds the godoc examples.

Tests that judge an expiry (a relay block, a measurement window) step an injected clock instead of sleeping, so a slow machine cannot make them pass or fail on timing.

## Property, metamorphic and differential tests

`property_test.go` draws inputs with `testing/quick` from the standard library, so the module keeps no dependencies, not even test-only ones. A failure prints the input that broke the property.

- `TestProperty_` tests state what holds for every input: a SOCKS5 CONNECT request for any domain of 1 to 255 bytes decodes back to that domain and port, a success rate is always in [0, 1] (NaN and infinities included), the metrics collector's counts, total, minimum, maximum and average agree with the requests it recorded, and the ADD_ONION command lists every port mapping once, in ascending order, on one line.
- `TestMetamorphic_` tests change the input in a way whose effect on the answer is known: relay statistics do not depend on the order of the measurements, a stricter threshold never makes a slow relay fast, `Reset` makes a collector behave like a new one, and circuit-status and stream-status lines parse the same with unknown keywords added, keywords reordered, or other whitespace.
- `TestDifferential_` tests compare with another implementation: the address bytes of a CONNECT request against `net/netip`, with a decoder written from RFC 1928 section 4, and the average relay latency against an exact mean computed with `math/big`.
- `TestBoundary_` tests pin the edges by hand: a domain of 0, 1, 254, 255 and 256 bytes, and ports -1, 0, 65535 and 65536.

The property tests found three bugs fixed in 0.5.0: a request measured as 0 was replaced as the minimum latency, `NewSuccessRate(NaN)` returned NaN, and the average relay latency wrapped around when the sum of the latencies overflowed a `time.Duration`.

```sh
go test . -short -run '^Test(Property|Metamorphic|Differential|Boundary)_'
go test . -short -run '^Test(Property|Metamorphic|Differential)_' -quickchecks=20000
```

The `Property` workflow (`.github/workflows/property.yml`) draws 20,000 inputs per test every night with a new random seed.

## Fuzzing

`fuzz_test.go` holds the targets: `FuzzReadReply` and `FuzzReadReplyRoundTrip` for the ControlPort reply parser, `FuzzQuotedString` and `FuzzSetConfSendsOneLine` for the commands sent to it, `FuzzParseStatusLines` for the circuit-status and stream-status parsers, `FuzzConsumeConnectReply` for the SOCKS5 reply reader (it must accept exactly the replies RFC 1928 section 6 describes and leave the bytes after the reply unread), and `FuzzConnectRequestRoundTrip` for the CONNECT request builder. `go test` runs their seed corpus; the `Fuzz` workflow searches each of them every night.

```sh
go test . -run '^$' -fuzz '^FuzzConsumeConnectReply$' -fuzztime 1m
```

## Mutation testing

[gremlins](https://github.com/go-gremlins/gremlins) v0.6.0 changes one comparison, bound or arithmetic operator at a time and runs the package's tests; a mutant no test notices is a line the tests execute without checking. The `Mutation` workflow (`.github/workflows/mutation.yml`) mutates the lines a pull request changes, and the whole package once a week. Tor is not installed in that job, so each mutant runs only the deterministic tests.

```sh
gremlins unleash --workers 4 --timeout-coefficient 30 .
git config diff.context 0; git config diff.interHunkContext 0   # one hunk per edit, so --diff sees every changed line
gremlins unleash --diff origin/main --workers 4 .
```

Measured on 2026-11-19, the first full run, with `tor` left out of `PATH` as in CI, `--workers 4 --timeout-coefficient 30` and a 4 GiB memory cap per process. A timed-out mutant is one whose tests no longer finish, counted apart from both:

| Package | Killed | Lived | Not covered | Timed out | Efficacy |
|---|---|---|---|---|---|
| `.` (the package) | 294 | 97 | 366 | 3 | 75.19% |

Most of the not-covered mutants are in the `examples/` programs, which have no tests, and in the code that only the Tor integration tests reach (`daemon.go`, `security.go`, `circuit.go`). The survivors are spread over `control.go` (23), `client.go` (23) and `relay.go` (19): mostly timeouts and retry delays whose exact value no test observes, which is where a test is worth adding next. A pull request should not lower the efficacy below this table.
