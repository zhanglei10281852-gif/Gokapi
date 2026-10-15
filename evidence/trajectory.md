# File Request 分块预约的共享配额与跨节点生命周期

> **Workspace:** F:\swe\098903\project-01

---

**User**

外部来宾可能通过同一个 File Request 同时选择文件，负载均衡器也会把 reserve、分块上传和完成请求分配到不同实例；此时 MaxFiles 必须把已上传文件与全部仍有效的预约视为一份共享配额，不能因为并发检查、节点切换或进程重启多放进文件，也不能让一个合法 UUID 换节点后突然失效。请修复这条上传生命周期：有限配额的最后名额只能被一个预约取得，上传开始后延长有效期，成功、取消、失败或超时都只释放一次，重复或迟到的完成与取消请求保持幂等；服务重启后仍应识别未过期预约并确定回收过期项，管理列表和剩余文件数始终反映已完成文件与有效预约的合计。无限文件数的请求、普通上传和现有单实例成功流程保持原有行为。

**Assistant**

I'll start by exploring the project structure to understand the codebase and the upload lifecycle that needs fixing.

toolName: view_folder

status: failed

f:\swe\098903\project-01

toolName: file_search

status: success

This is the Gokapi Go project. Let me explore the file request and upload-related code.

toolName: file_search

status: success

Let me read the key files related to file requests, chunk reservations, and uploads.

toolName: view_files

status: success

filePath: f:\swe\098903\project-01\internal\storage\chunking\chunkreservation\ChunkReservation.go

toolName: view_files

status: success

filePath: f:\swe\098903\project-01\internal\storage\filerequest\Filerequest.go

toolName: view_files

status: success

filePath: f:\swe\098903\project-01\internal\models\FileRequest.go

Now let me look at the upload handler and chunking logic:

toolName: view_files

status: success

filePath: f:\swe\098903\project-01\internal\webserver\fileupload\FileUpload.go

toolName: view_files

status: success

filePath: f:\swe\098903\project-01\internal\storage\chunking\Chunking.go

Now let me find where reservations are created and how upload status flows:

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: f:\swe\098903\project-01\internal\models\UploadStatus.go

Let me read the API handler where reservations are made and completed:

toolName: view_files

status: success

filePath: f:\swe\098903\project-01\internal\webserver\api\Api.go

Let me examine the database layer to understand persistence options:

toolName: view_files

status: success

filePath: f:\swe\098903\project-01\internal\configuration\database\Database.go

toolName: view_files

status: success

filePath: f:\swe\098903\project-01\internal\configuration\database\dbabstraction\DbAbstraction.go

toolName: view_files

status: success

filePath: f:\swe\098903\project-01\internal\configuration\database\provider\sqlite\filerequests.go

toolName: view_files

status: success

filePath: f:\swe\098903\project-01\internal\configuration\database\provider\redis\filerequests.go

toolName: view_files

status: success

filePath: f:\swe\098903\project-01\internal\configuration\database\provider\sqlite\Sqlite.go

toolName: view_files

status: success

filePath: f:\swe\098903\project-01\internal\configuration\database\provider\redis\Redis.go

Let me check the existing tests, rate limiter, and how GC is invoked at startup:

toolName: view_files

status: success

filePath: f:\swe\098903\project-01\internal\storage\chunking\chunkreservation\ChunkReservation_test.go

toolName: view_files

status: success

filePath: f:\swe\098903\project-01\internal\webserver\ratelimiter\RateLimiter.go

toolName: search_by_regex

status: success

Let me look at the GC scheduling and the frontend upload flow, plus how files complete (`NewFileFromChunk`):

toolName: view_files

status: success

filePath: f:\swe\098903\project-01\internal\storage\FileServing.go

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: f:\swe\098903\project-01\internal\storage\FileServing.go

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: f:\swe\098903\project-01\internal\storage\FileServing.go

Let me look at the startup flow, Redis metadata keys, SQLite session GC pattern, and test infrastructure:

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: f:\swe\098903\project-01\internal\configuration\database\provider\redis\metadata.go

toolName: view_files

status: success

filePath: f:\swe\098903\project-01\internal\configuration\database\provider\sqlite\sessions.go

toolName: view_files

status: success

filePath: f:\swe\098903\project-01\cmd\gokapi\Main.go

toolName: view_files

status: success

filePath: f:\swe\098903\project-01\internal\test\TestHelper.go

Let me check how tests initialize databases and the schema version test references:

toolName: view_files

status: success

filePath: f:\swe\098903\project-01\internal\configuration\database\Database_test.go

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: f:\swe\098903\project-01\internal\storage\FileServing.go

toolName: view_files

status: success

filePath: f:\swe\098903\project-01\internal\webserver\web\static\js\public_upload.js

Let me find the request param structs for the chunk endpoints:

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: f:\swe\098903\project-01\internal\webserver\api\Api.go

toolName: search_by_regex

status: success

toolName: search_by_regex

status: success

Let me check the DB test helpers and Redis metadata save/delete implementations:

toolName: view_files

status: success

filePath: f:\swe\098903\project-01\internal\configuration\database\Database_test.go

toolName: view_files

status: success

filePath: f:\swe\098903\project-01\internal\configuration\database\provider\redis\metadata.go

I have a complete picture of the problem now. Let me set up a task plan and verify a couple of remaining details.

toolName: todo_write

status: success

Todos updated: 9 items

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: f:\swe\098903\project-01\internal\models\FileList.go

No import cycle risk. Now let me implement. First, the refactored `chunkreservation` package:

toolName: Write

status: success

filePath: f:\swe\098903\project-01\internal\storage\chunking\chunkreservation\ChunkReservation.go

changes: undefined

Now extend the database abstraction interface:

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-01\internal\configuration\database\dbabstraction\DbAbstraction.go

changes: undefined

toolName: todo_write

status: success

Todos updated: 9 items

Now the SQLite provider implementation:

toolName: Write

status: success

filePath: f:\swe\098903\project-01\internal\configuration\database\provider\sqlite\chunkreservations.go

changes: undefined

Now update the SQLite schema (version 16), new-DB creation, upgrade path, and cascade delete:

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-01\internal\configuration\database\provider\sqlite\Sqlite.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-01\internal\configuration\database\provider\sqlite\Sqlite.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-01\internal\configuration\database\provider\sqlite\Sqlite.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-01\internal\configuration\database\provider\sqlite\filerequests.go

changes: undefined

toolName: todo_write

status: success

Todos updated: 9 items

Now the Redis provider implementation:

toolName: Write

status: success

filePath: f:\swe\098903\project-01\internal\configuration\database\provider\redis\chunkreservations.go

changes: undefined

Let me simplify the Redis cleanup logic to be clearer and correct:

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-01\internal\configuration\database\provider\redis\chunkreservations.go

changes: undefined

Now maintain the completed-files index in Redis metadata save/delete, and add cascade deletion:

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-01\internal\configuration\database\provider\redis\metadata.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-01\internal\configuration\database\provider\redis\filerequests.go

changes: undefined

Now bump the Redis schema to v9 and backfill the file index on upgrade:

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-01\internal\configuration\database\provider\redis\Redis.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-01\internal\configuration\database\provider\redis\Redis.go

changes: undefined

Now add the database-package wrappers and the DB-backed store adapter:

toolName: todo_write

status: success

Todos updated: 9 items

toolName: Write

status: success

filePath: f:\swe\098903\project-01\internal\configuration\database\chunkreservations.go

changes: undefined

Now hook expired-reservation reclamation into the periodic database GC:

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-01\internal\configuration\database\Database.go

changes: undefined

Now wire the store init into Main.go:

toolName: view_files

status: success

filePath: f:\swe\098903\project-01\cmd\gokapi\Main.go

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-01\cmd\gokapi\Main.go

changes: undefined

toolName: todo_write

status: success

Todos updated: 9 items

Now fix the API lifecycle handlers:

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-01\internal\webserver\api\Api.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-01\internal\webserver\api\Api.go

changes: undefined

Now release the reservation when chunk processing fails after upload has started:

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-01\internal\webserver\fileupload\FileUpload.go

changes: undefined

Now let me check whether any test mocks implement the `Database` interface that would need the new methods, then build:

toolName: search_by_regex

status: success

toolName: search_by_regex

status: success

Only the two providers implement the interface. Let me build the project to catch compile errors:

toolName: Shell

status: success

The WASM embed error is a pre-existing issue with generated assets. Let me build the packages I changed directly:

toolName: Shell

status: success

toolName: Shell

status: success

The WASM embed is a pre-existing generated-asset issue. Let me check how tests are run and then run the relevant test suites:

toolName: view_files

status: success

filePath: f:\swe\098903\project-01\makefile

Tests run with `--tags=test,awsmock`. Let me check the embed directive that fails and how tests handle it:

toolName: view_files

status: success

filePath: f:\swe\098903\project-01\internal\webserver\Webserver.go

toolName: file_search

status: success

toolName: view_files

status: success

filePath: f:\swe\098903\project-01\.gitignore

The `.wasm` files are gitignored build artifacts. I'll create temporary placeholders so the embed compiles during local testing:

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

Existing tests pass. Now run the database tests (SQLite + miniredis) — this also validates the Lua scripts and new provider methods:

toolName: Shell

status: success

All database tests pass, including the Lua scripts under miniredis. Now let me add tests. First, lifecycle tests for the in-memory store:

toolName: Write

status: success

filePath: f:\swe\098903\project-01\internal\storage\chunking\chunkreservation\ChunkReservation_lifecycle_test.go

changes: undefined

Update `SetStore` so `nil` resets to the in-memory store (used by tests):

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-01\internal\storage\chunking\chunkreservation\ChunkReservation.go

changes: undefined

toolName: Shell

status: success

toolName: Shell

status: success

The stub's bookkeeping is wrong — fix it:

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-01\internal\storage\chunking\chunkreservation\ChunkReservation_lifecycle_test.go

changes: undefined

Now add a database-level parity test. Let me check the existing file-request tests for placement:

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: f:\swe\098903\project-01\internal\configuration\database\Database_test.go

Now add the database parity test covering both SQLite and Redis, including end-to-end store wiring:

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-01\internal\configuration\database\Database_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-01\internal\configuration\database\Database_test.go

changes: undefined

Now run the new database test across both providers:

toolName: Shell

status: success

Both providers pass. Now run the broader test suite to check for regressions:

toolName: todo_write

status: success

Todos updated: 9 items

toolName: Shell

status: success

Some failures appear (Windows path/file-lock/cert issues). Let me get the complete list of failing packages to distinguish pre-existing issues from regressions:

toolName: Shell

status: success

The first full run showed `webserver/api` passing, while the second shows different failures — suggesting flaky, environment-dependent tests (Windows file locks, port conflicts). Let me run the suspicious packages individually to check determinism:

toolName: Shell

status: success

These look like environment-related failures (file I/O panics, nil server). Let me establish a baseline by stashing all my changes and running the same tests:

toolName: Shell

status: success

The stash included untracked test artifacts too. Note: `ChunkReservation_test.go` was modified by my gofmt run — I'll review that after. Now run the baseline tests:

toolName: Shell

status: success

Test DB files created by my v16 run are lingering (and confusing baseline v15 code). Let me clean all test artifacts and rerun the baseline:

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

My cleanup accidentally deleted the tracked `internal/test` helper package. Restore it from git:

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

Changes restored. Let me check what gofmt changed in the original test file:

toolName: Shell

status: success

No actual change (line-ending normalization only). Now clean stale test DBs and rerun the affected packages with my changes:

toolName: Shell

status: success

All affected packages pass. Now run the complete suite with sequential packages (`-p 1`) to get a deterministic result set:

toolName: Shell

status: success

Sequential full-suite results show only the same pre-existing Windows/environment failures (file locking in storage/chunking, path separators in localstorage, port-in-use, cert date). Notably `cmd/gokapi` passes. Let me verify how the admin UI displays remaining files, to ensure the reserved-count reaches the management view:

toolName: search_by_regex

status: success

toolName: search_by_regex

status: success

toolName: search_by_regex

status: success

The data flow is correct end-to-end. Let me verify `outputFileJson` is nil-safe (it's called from the non-blocking goroutine with nil writer, including my new duplicate branch):

toolName: search_by_regex

status: success

Nil-safe. Let me check existing API tests for the reserve/complete endpoints to see if end-to-end coverage exists:

toolName: search_by_regex

status: success

No existing HTTP tests for these endpoints; the wiring is covered by DB-level integration tests. Let me run vet and gofmt checks:

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

The gofmt flags are repo-wide CRLF effects (even untouched files like FileRequest.go are flagged). Let me verify my changed files don't contain whole-file line-ending churn vs git index:

toolName: Shell

status: success

Let me make one consistency refinement: the Redis file index should exclude pending-deletion files (matching the SQLite `PendingDeletion=0` predicate), including during backfill:

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-01\internal\configuration\database\provider\redis\metadata.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-01\internal\configuration\database\provider\redis\Redis.go

changes: undefined

Let me strengthen the parity test with a pending-deletion quota case:

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-01\internal\configuration\database\Database_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-01\internal\configuration\database\Database_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-01\internal\configuration\database\Database_test.go

changes: undefined

toolName: Shell

status: success

Let me do a final review of the complete diffs:

toolName: Shell

status: success

The diffs look correct. Let me review the final reservation package and the new DB files in full to ensure quality:

toolName: view_files

status: success

filePath: f:\swe\098903\project-01\internal\storage\chunking\chunkreservation\ChunkReservation.go

Let me run a full build now that placeholder wasm files exist, to verify every package including cmd:

toolName: Shell

status: success

Full build passes. Let me verify concurrency safety with the race detector:

toolName: Shell

status: success

Race-clean. Let me verify the management API exposes the combined counts when listing file requests:

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: f:\swe\098903\project-01\internal\webserver\api\Api.go

All management endpoints serialize `uploadedfiles` + `reserveduploads` via `Populate()`, now backed by the shared store. Implementation is complete. Let me clean up local test artifacts and placeholder wasm files to leave the workspace pristine:

toolName: Shell

status: success

Workspace is clean with only intended changes. Final verification run of the core packages that don't depend on generated wasm assets:

toolName: Shell

status: success

All my changed packages pass. The only failure is `setup.TestIsErrorAddressAlreadyInUse` — a pre-existing Windows network-probe failure in a package I didn't touch (it also failed in the baseline runs).

toolName: todo_write

status: success

Todos updated: 9 items

已修复这条上传生命周期。核心问题是预约（reservation）原来只存在单进程内存 map 里：检查与插入分离（并发可超卖）、换节点/重启即失效、失败不释放、完成不可重放。现在预约持久化到共享数据库并以原子操作管理。

## 改动概览

**预约抽象与默认实现** — [ChunkReservation.go](file:///f:/swe/098903/project-01/internal/storage/chunking/chunkreservation/ChunkReservation.go)
- 抽出 `Store` 接口；默认仍是原内存实现（单实例/单元测试行为不变），生产环境注入数据库实现。
- 新增原子 `TryNew(maxFiles, uploadedFiles)`：在同一把锁/同一条语句内判定“已完成文件 + 有效预约”合计是否还有名额，最后一个名额只会被一个预约取得；50 并发抢 1 个名额的测试通过。
- `Finalise/GetFinalised`：成功后预约转为 finalised 并保留至到期，重复或迟到的完成请求可幂等返回原文件。
- `SetComplete/Release` 对成功、取消、失败、超时都是同一条幂等删除，任何终端事件最多释放一次；`GetCount` 只统计未过期的有效预约。

**数据库层（SQLite + Redis 双实现）**
- 接口扩展：[DbAbstraction.go](file:///f:/swe/098903/project-01/internal/configuration/database/dbabstraction/DbAbstraction.go#L112-L132)
- SQLite：新表 `ChunkReservations`，[sqlite/chunkreservations.go](file:///f:/swe/098903/project-01/internal/configuration/database/provider/sqlite/chunkreservations.go) 用单条条件 `INSERT … SELECT WHERE 已完成数+有效预约数 < MaxFiles` 保证原子配额（同时统计 `PendingDeletion=0` 的文件）；schema 升级到 v16（新库建表 + 老库升级）。
- Redis：[redis/chunkreservations.go](file:///f:/swe/098903/project-01/internal/configuration/database/provider/redis/chunkreservations.go) 用 ZSET（有效/已完成）+ HASH（uuid→fileId）+ SET（已完成文件索引），配额检查与插入、完成迁移均由 Lua 脚本原子完成；文件保存/删除/待删状态自动维护索引，schema v9 升级时回填；版本升级到 9。
- 删除文件请求时级联清理其全部预约；[Database.go](file:///f:/swe/098903/project-01/internal/configuration/database/Database.go#L99-L103) 的 GC 每小时回收过期预约。

**装配与回收** — [database/chunkreservations.go](file:///f:/swe/098903/project-01/internal/configuration/database/chunkreservations.go)：`dbReservationStore` 适配器 + `InitChunkReservationStore()`；[Main.go](file:///f:/swe/098903/project-01/cmd/gokapi/Main.go#L59-L63) 在连接数据库后调用——重启后未过期预约依然被识别，过期项启动时立即回收，且任意节点都认同一 uuid。

**HTTP 生命周期** — [Api.go](file:///f:/swe/098903/project-01/internal/webserver/api/Api.go#L384-L424)
- reserve：有限配额走原子 `TryNew`（无名额返回 400），无限文件数保留原限流 `IsAllowedNewUuid` + 旧路径。
- complete：成功只 Finalise 一次；重复/迟到完成直接返回首次创建的文件（200，幂等）；真正失败时恰好释放一次配额。
- 分块开始上传仍延长有效期至 23 小时；[FileUpload.go](file:///f:/swe/098903/project-01/internal/webserver/fileupload/FileUpload.go#L96-L104) 在分块写入失败时释放预约。
- cancel/unreserve 维持幂等 OK。

**管理视图**：`Populate()` 的 `ReservedUploads` 现在走共享库统计，[FileRequest.FilesRemaining()](file:///f:/swe/098903/project-01/internal/models/FileRequest.go#L97-L103) 与管理列表 JSON（`uploadedfiles`/`reserveduploads`）、来宾页剩余名额始终是“已完成文件 + 有效预约”的合计。

## 测试
- 新增内存生命周期测试（配额原子性、50 并发抢末位、成功幂等、只释放一次、store 注入）与跨 SQLite/Redis 的数据库对等测试（配额、续期、幂等完成、过期回收、级联删除、待删文件、`SetStore`+`Populate` 集成）；`-race` 通过。
- 全量顺序测试中我改动相关的包全部通过。仅余与本次无关的既有 Windows 环境失败（storage/chunking 的 Windows 文件占用、localstorage 的 `\`/`/` 路径分隔符、setup 端口探测、ssl 证书日期断言），已通过基线对照确认。

说明：本地测试需 `go generate` 产生的 `web/*.wasm`（gitignored），我在验证后已删除临时占位文件，工作区只保留本次代码改动。
