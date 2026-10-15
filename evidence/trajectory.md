# 多实例 Data Served 精确累计与重置隔离

> **Workspace:** F:\swe\098903\project-02

---

**User**

管理员从任一 Gokapi 实例打开状态页时，Data Served 应表示整个共享部署自最近一次成功重置以来实际计量的下载量，而不是当前节点最后保存的一份内存快照。修复现有统计链路，使多个节点同时完成下载、交错执行定期保存或优雅关闭时，各自增量都精确保留；节点异常退出或重新启动后继续使用同一共享基线，不重复也不覆盖其他节点的计数。管理员重置统计与下载并发发生时，要明确区分重置前后的流量，旧节点随后保存或关闭不能让重置前的数值重新出现，重置后的下载仍正常累计；状态页和 /api/logs/systemstatus 无论命中哪个实例，都返回一致的总量和 recording since。单实例、零流量以及现有本地和代理下载行为不得回归。

**Assistant**

toolName: view_folder

status: failed

f:\swe\098903\project-02

toolName: file_search

status: success

This is the Gokapi Go codebase. Let me explore the statistics-related code first.

toolName: view_files

status: success

filePath: f:\swe\098903\project-02\internal\logging\serverstats\ServerStats.go

toolName: search_by_regex

status: success

toolName: file_search

status: success

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: f:\swe\098903\project-02\internal\configuration\database\provider\redis\statistics.go

toolName: view_files

status: success

filePath: f:\swe\098903\project-02\internal\configuration\database\provider\sqlite\statistics.go

toolName: view_files

status: success

filePath: f:\swe\098903\project-02\internal\configuration\database\dbabstraction\DbAbstraction.go

toolName: view_files

status: success

filePath: f:\swe\098903\project-02\internal\webserver\Webserver.go

toolName: view_files

status: success

filePath: f:\swe\098903\project-02\internal\webserver\api\Api.go

toolName: view_files

status: success

filePath: f:\swe\098903\project-02\internal\configuration\database\Database.go

toolName: view_files

status: success

filePath: f:\swe\098903\project-02\internal\storage\FileServing.go

toolName: search_by_regex

status: success

toolName: search_by_regex

status: success

toolName: search_by_regex

status: success

toolName: search_by_regex

status: success

toolName: file_search

status: success

toolName: search_by_regex

status: success

toolName: Shell

status: success

toolName: file_search

status: success

toolName: search_by_regex

status: success

toolName: search_by_regex

status: success

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: f:\swe\098903\project-02\cmd\demoData\demoData.go

toolName: view_files

status: success

filePath: f:\swe\098903\project-02\internal\configuration\database\provider\redis\metadata.go

toolName: view_files

status: success

filePath: f:\swe\098903\project-02\internal\configuration\database\provider\sqlite\metadata.go

toolName: view_files

status: success

filePath: f:\swe\098903\project-02\internal\configuration\database\provider\redis\Redis.go

toolName: search_by_regex

status: success

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: f:\swe\098903\project-02\internal\configuration\database\provider\sqlite\Sqlite.go

toolName: Shell

status: success

toolName: view_files

status: success

filePath: f:\swe\098903\project-02\internal\test\testconfiguration\TestConfiguration.go

Now let me examine the test conventions and the remaining call sites.

toolName: view_files

status: success

filePath: f:\swe\098903\project-02\internal\webserver\downloadstatus\DownloadStatus_test.go

toolName: view_files

status: success

filePath: f:\swe\098903\project-02\internal\configuration\database\provider\sqlite\Sqlite_test.go

toolName: view_files

status: success

filePath: f:\swe\098903\project-02\internal\logging\Logging_test.go

toolName: view_files

status: success

filePath: f:\swe\098903\project-02\internal\configuration\database\Database_test.go

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: f:\swe\098903\project-02\internal\configuration\database\provider\redis\Redis_test.go

toolName: view_files

status: success

filePath: f:\swe\098903\project-02\internal\webserver\Webserver.go

toolName: search_by_regex

status: success

toolName: Shell

status: success

Let me view the test harness helpers and the frontend logs page briefly, then set up a task plan.

toolName: view_files

status: success

filePath: f:\swe\098903\project-02\internal\configuration\database\Database_test.go

toolName: view_files

status: success

filePath: f:\swe\098903\project-02\internal\webserver\web\templates\html_logs.tmpl

toolName: todo_write

status: success

Todos updated: 5 items

toolName: view_files

status: success

filePath: f:\swe\098903\project-02\makefile

toolName: view_files

status: success

filePath: f:\swe\098903\project-02\internal\test\TestHelper.go

Now implementing. First the database abstraction interface:

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-02\internal\configuration\database\dbabstraction\DbAbstraction.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-02\internal\configuration\database\Database.go

changes: undefined

Now the SQLite provider:

toolName: Write

status: success

filePath: f:\swe\098903\project-02\internal\configuration\database\provider\sqlite\statistics.go

changes: undefined

Now the Redis provider:

toolName: Write

status: success

filePath: f:\swe\098903\project-02\internal\configuration\database\provider\redis\statistics.go

changes: undefined

Now the core rewrite of serverstats:

toolName: Write

status: success

filePath: f:\swe\098903\project-02\internal\logging\serverstats\ServerStats.go

changes: undefined

Now wire the graceful shutdown in the webserver:

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-02\internal\webserver\Webserver.go

changes: undefined

toolName: todo_write

status: success

Todos updated: 5 items

toolName: Shell

status: success

toolName: file_search

status: success

toolName: Shell

status: success

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: f:\swe\098903\project-02\internal\storage\FileServing_test.go

Now let me add tests. First, check the end of the provider test files:

toolName: Shell

status: success

toolName: view_files

status: success

filePath: f:\swe\098903\project-02\internal\configuration\database\provider\sqlite\Sqlite_test.go

toolName: view_files

status: success

filePath: f:\swe\098903\project-02\internal\configuration\database\provider\redis\Redis_test.go

toolName: view_files

status: success

filePath: f:\swe\098903\project-02\internal\configuration\database\provider\sqlite\Sqlite_test.go

Now create the multi-node serverstats test:

toolName: Write

status: success

filePath: f:\swe\098903\project-02\internal\logging\serverstats\ServerStats_test.go

changes: undefined

Now append provider-level tests:

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-02\internal\configuration\database\provider\sqlite\Sqlite_test.go

changes: undefined

toolName: view_files

status: success

filePath: f:\swe\098903\project-02\internal\configuration\database\provider\redis\Redis_test.go

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-02\internal\configuration\database\provider\redis\Redis_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-02\internal\configuration\database\Database_test.go

changes: undefined

toolName: todo_write

status: success

Todos updated: 5 items

toolName: Shell

status: success

Two issues to fix: missing `sync` import and SQLITE_BUSY under concurrent transactions. Let me check modernc sqlite driver capabilities:

toolName: search_by_regex

status: success

toolName: Shell

status: success

toolName: Shell

status: success

The driver supports `_txlock=immediate`, which makes concurrent write transactions queue via busy_timeout instead of deadlocking. Applying it at the connection DSN:

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-02\internal\configuration\database\provider\sqlite\Sqlite.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-02\internal\configuration\database\Database_test.go

changes: undefined

toolName: Shell

status: success

All green. Now race detection and repeated runs to validate the concurrency:

toolName: Shell

status: success

Those failures are pre-existing test-harness state leakage under `-count=3` (shared sqlite file / miniredis data reused across iterations) — note serverstats passed 3× cleanly. Let me validate my packages with single-count race runs, one at a time:

toolName: Shell

status: success

The sqlite provider failure is stale `test/` data from the earlier aborted `-count=3` panic run. Cleaning and re-running:

toolName: Shell

status: success

Now run the full test suite as the project does. First generate embedded assets:

toolName: Shell

status: success

toolName: Shell

status: success

Build passed. The 3 failing packages are pre-existing Windows-environment issues (path separators, file locks, cert dates) unrelated to statistics. Let me confirm the complete failure list and re-run those three against pristine code expectations:

toolName: Shell

status: success

Let me verify these failures are pre-existing by stashing my changes and re-running the failing packages:

toolName: Shell

status: success

`go generate` touched generated assets too — I'll review/revert those afterward. First, baseline-check the failing packages with my changes stashed:

toolName: Shell

status: success

toolName: Shell

status: success

Confirmed — all 7 packages fail identically on the pristine baseline (pre-existing Windows environment issues). Restoring my fix:

toolName: Shell

status: success

Now let me inspect the go-generate-spawned changes and revert anything unrelated to my fix:

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

toolName: delete_file

status: success

Final checks — gofmt, vet, and inspect other SQLite transaction users that now also get `BEGIN IMMEDIATE`:

toolName: search_by_regex

status: success

toolName: Shell

status: success

gofmt lists many untouched files too — likely CRLF noise. Let me verify whether my edited files have real formatting issues versus pre-existing line-ending noise:

toolName: Shell

status: success

Pure CRLF noise affecting the whole repo equally — my files are stylistically consistent. Final clean verification run:

toolName: Shell

status: success

All affected packages pass; remaining failures are the pre-existing Windows baseline issues confirmed earlier. Let me do a final review of the complete core diff:

toolName: Shell

status: success

toolName: view_files

status: success

filePath: f:\swe\098903\project-02\internal\logging\serverstats\ServerStats.go

toolName: todo_write

status: success

Todos updated: 5 items

## 修复总结

### 根因

旧链路是"节点内存快照"模型：每个节点启动时把共享库里的总量读进内存（`Init → AddTraffic(GetStatTraffic())`），之后只在本地累加，定期保存/关闭时用 `SaveStatTraffic(绝对总量)` 整体覆盖写回。因此：

- 多节点并发下载时，各自保存的绝对量互相覆盖（丢失更新）；
- 某节点重置后，其他节点内存里仍持有重置前的总量，下次定期保存或关闭会把旧值"复活"；
- `recording since` 只缓存在各节点内存，跨实例不一致；
- 节点崩溃丢失最近 5 分钟增量，重启还会用旧基线覆盖他人计数。

### 修复方案：共享基线 + 按纪元（epoch）归属的条件增量

[ServerStats.go](file:///f:/swe/098903/project-02/internal/logging/serverstats/ServerStats.go) 重写为每个节点只维护**自己尚未落库的增量**，并按下载完成时读到的共享 epoch 分桶（`map[epoch]bytes`），不再把共享总量装入内存：

- **下载计量**：[addTraffic](file:///f:/swe/098903/project-02/internal/logging/serverstats/ServerStats.go#L137-L153) 每次先读共享 epoch，再把字节数计入对应桶；重置前后的流量在归属点即被明确区分。
- **定期保存/优雅关闭**：[saveTraffic](file:///f:/swe/098903/project-02/internal/logging/serverstats/ServerStats.go#L158-L179) 用节点级锁串行化，把每个桶**条件增量**写回共享库——仅当库里 epoch 与桶一致时才 `INCR`，否则丢弃。交错保存只加不盖，互不覆盖；崩溃最多丢失本节点未落库增量，重启继续使用同一基线，不重复、不覆盖。
- **重置**：[clearTraffic](file:///f:/swe/098903/project-02/internal/logging/serverstats/ServerStats.go#L181-L200) 先丢弃本节点全部待存增量，再**一个原子事务**把共享总量置 0、epoch 置新值。旧节点随后保存/关闭时，旧 epoch 的增量被条件写拒绝，绝不会让重置前数值重现；重置后读到新 epoch 的下载照常累计。
- **状态页/API 一致性**：[getCurrentTraffic](file:///f:/swe/098903/project-02/internal/logging/serverstats/ServerStats.go#L202-L212) 先 flush 本节点增量，再读共享总量（加上当前 epoch 的在途增量）和共享 epoch，任意实例返回一致的 Data Served 与 recording since。
- 零流量无任何写操作；`AddTraffic(0)` 直接返回，单实例与本地/代理下载调用路径完全不变。

### 数据库原语

- [sqlite/statistics.go](file:///f:/swe/098903/project-02/internal/configuration/database/provider/sqlite/statistics.go)：`AddStatTraffic` 用事务"比对 epoch → UPSERT `value=value+?`"，`ResetStatTraffic` 事务内同时归零总量和更新 epoch，`InitTrafficSince` 仅在缺失时创建。
- [redis/statistics.go](file:///f:/swe/098903/project-02/internal/configuration/database/provider/redis/statistics.go)：同一语义用 Lua 脚本原子完成（`GET epoch → 条件 INCRBY`），重置用 `MULTI/EXEC`。
- 接口与封装同步更新：[DbAbstraction.go](file:///f:/swe/098903/project-02/internal/configuration/database/dbabstraction/DbAbstraction.go#L112-L126)、[Database.go](file:///f:/swe/098903/project-02/internal/configuration/database/Database.go#L363-L390)，删除了旧的绝对量写接口。
- [Sqlite.go](file:///f:/swe/098903/project-02/internal/configuration/database/provider/sqlite/Sqlite.go#L154) DSN 增加 `_txlock=immediate`，使多节点/多连接写事务在 busy_timeout 下排队，消除并发写升级死锁导致的 `SQLITE_BUSY`。
- [Webserver.go](file:///f:/swe/098903/project-02/internal/webserver/Webserver.go#L198-L206) 优雅关闭时在 HTTP 服务停止后调用 `serverstats.Shutdown()`，确保关闭前增量落库。

### 测试

- 新增 [ServerStats_test.go](file:///f:/swe/098903/project-02/internal/logging/serverstats/ServerStats_test.go)：零流量、单实例、双节点交错保存、重启沿用基线不重复、第三节点重置后旧节点保存/关闭不复活旧值且重置后下载正常累计、全局重置丢弃他节点待存增量、关闭落盘、400 个并发下载/保存竞态。
- 两个 provider 各增 `TestStatistics`（含全新库 epoch 自动建立、过期 epoch 拒绝、重置原子性），[Database_test.go](file:///f:/swe/098903/project-02/internal/configuration/database/Database_test.go#L372-L429) 增加双后端条件增量与 50 并发写竞态测试。
- 全部受影响包在 `-tags=test` 与 `-race` 下通过；全量 `go build ./...` 通过。全量 `go test` 中仅有的 7 个失败包（路径分隔符、Windows 文件锁、证书日期等）经 stash 对照验证在改动前同样失败，与本次修改无关。
