# Java 25 migration

CoTriage-Nova is an independent Java migration of CoTriage-dev commit
`c04802d3ea9fadb765d3a0d9dc6d87202330fc21`. The source checkout is preserved.
The original 1,452 tracked files and their SHA-256 hashes are recorded in
[source-manifest.json](source-manifest.json).

## Scope and contracts

The five top-level services are Java 25 / Spring Boot 4.1.1 applications.
Vue, Python model services, the optional avatar integrations and
`cotrige-archive` retain their original source. Historical reports, images and
experiment claims are not evidence for this Java implementation.

The common module contains wire JSON helpers, Go-duration parsing, legacy MySQL
DSN adaptation, Redis/Kubernetes client setup and additive schema initialization.
There is no global snake_case/null normalization. Core wire formats, table names,
stream/group names, RAISE constants, transaction boundaries and ACK ordering
follow the Go implementation. The server continues to expose all 47 REST routes
and the `inferred` / `routed` SSE streams.

The tests include Go-derived inference and RAISE fixtures, isolated Redis/MySQL,
HTTP model fixtures and a separate `migration-tests` module which launches all
five packaged JVM processes. Test model responses establish orchestration
behavior, not model quality.

## Deliberately retained limitations

This migration does not claim stronger reliability than the Go source.

- Collector starts at the current end of the file, does not implement rotation,
  discards a partial line at EOF, and skips a complete line after XADD failure.
- Edge inference preserves its existing Pending reclaim and retry/DLQ behavior.
  Its publish / processed marker / ACK steps remain non-atomic.
- The original local unconfigured model interface can fail as a typed-nil call;
  the Java compatibility sentinel retains the equivalent failed processing path.
- Remote `all_keywords` rules generate lookahead expressions rejected by the
  original Go regular expression engine. Java retains that rejection.
- Router and cloud gateway do not add Pending reclaim. Retryable cloud errors
  may remain in the Redis Pending Entries List.
- Outbox SENT means that the Redis request was published, not that cloud
  diagnosis completed. Deadlines affect sorting; they do not cancel requests.
  Replayer does not add a new cloud-health gate.
- Gateway lookup / model invocation / feedback insertion are separate steps.
  Duplicate concurrent model calls remain possible.
- Feedback claim, in-memory learning, reward update and snapshot persistence
  preserve their existing separate commit boundaries and crash windows.
- Queue pressure retains XLEN, affected devices retains distinct Source, and
  recent cloud ratio retains cumulative counters. Neither RAISE nor its two-stage
  reward calculations have been redesigned.
- Management authentication remains limited to the optional cloud-config bearer
  token. Provider API-key field behavior is retained.
- PATCH null semantics follow actual Go decoding, including pointer cases where
  null behaves like omission, rather than comments describing intended clearing.

## Runtime and database differences

JVM scheduling, stack traces, memory footprint and actual timing differ from Go.
Deterministic fixtures require exact action/reason/wire values and numerical
tolerance of 1e-12 for RAISE. Variable timestamps, IDs and observed elapsed times
are validated by format and relationships rather than identical wall-clock values.

`Schema.ensure` uses the source-derived DDL in
[fixtures/go-schema.sql](fixtures/go-schema.sql). It creates missing tables,
columns and named indexes without dropping rows or columns. Existing column type
changes are not automatically rewritten; the checked baseline's schema is the
compatibility target. Connector/J has no independent Go-style writeTimeout;
read and connection timeouts are mapped explicitly.

Jackson 2.20.1 handles compatibility payloads. Jackson annotations 2.21 are shared
with Spring Boot's Jackson 3 MVC infrastructure to avoid class linkage errors.
Maven settings are local to this project and use HTTPS Maven Central; commands
are run from the repository root.

## Verification and deployment

The completed validation record is in [validation-report.md](validation-report.md).
The optional image startup check uses only disposable fixtures:
`./script/test-java-images.ps1`; it removes its containers and network afterward.

From the repository root:

```powershell
.\mvnw.cmd verify
npm --prefix web ci
npm --prefix web run build
.\script\verify-migration.ps1
.\script\verify-migration.ps1 -CheckSource
.\script\render-java-manifests.ps1 -OutputPath .migration/java-manifests.yaml
```

`verify` requires an accessible Docker engine for disposable MySQL/Redis.
Container unavailability is a failure, not an automatic skipped success.
No verification command uses the existing Kubernetes cluster or cloud API keys.

Build images on a Docker host with:
```bash
./script/build-java-images.sh
# optional override
IMAGE_PREFIX=registry.example/team IMAGE_TAG=java25 ./script/build-java-images.sh
```

The Kubernetes resource names, mounts, RBAC and Secret references are retained.
The Java image prefix/tag can be rendered with
`script/render-java-manifests.ps1`. No images are pushed by the build helper.

The root startup script deliberately retains the source's FLUSHDB and collector
restart steps. Run it only against the intended development cluster; it is not a
read-only verification command. Actual cluster rollout and real model-quality
verification are outside this migration delivery.
