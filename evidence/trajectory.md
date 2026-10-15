# 登录 Session 单后继轮换与撤销防复活

> **Workspace:** F:\swe\098903\project-03

---

**User**

同一名已登录用户在会话到达 RenewAt 后同时发出多个请求时，当前实现会让每个请求各建一个新 Session；更严重的是，用户刚完成登出，或管理员刚完成密码重置、用户删除和按用户注销全部会话，先前已经读到旧 Session 的迟到请求仍可能把登录状态重新写回。请让会话轮换成为单后继的状态转换：同一个旧 ID 在一个续期点最多留下一个有效后继，并发响应可以安全取得同一轮换结果或要求重新认证，但不能产生多个长期有效 cookie，旧 ID 在转换后不可继续使用。上述本地撤销动作一旦生效，任何在此之前读到旧状态的续期都不得重新创建有效会话；SQLite 与 Redis、多实例切换应遵守相同语义。未到 RenewAt 的普通会话、超过 ValidUntil 的拒绝逻辑、OIDC 会话有效期和现有 cookie 属性保持不变。

**Assistant**

我先了解项目结构并回顾相关经验。

toolName: view_folder

status: failed

f:\swe\098903\project-03

该经验与当前任务关联不大。让我探索项目结构。

toolName: file_search

status: success

这是 Gokapi 项目（Go 文件分享服务器）。让我查看会话管理的核心代码。

toolName: view_files

status: success

filePath: f:\swe\098903\project-03\internal\webserver\authentication\sessionmanager\SessionManager.go

toolName: file_search

status: success

让我查看会话模型、数据库抽象层和两个 provider 的会话实现。

toolName: view_files

status: success

filePath: f:\swe\098903\project-03\internal\models\Session.go

toolName: view_files

status: success

filePath: f:\swe\098903\project-03\internal\configuration\database\Database.go

toolName: view_files

status: success

filePath: f:\swe\098903\project-03\internal\configuration\database\provider\sqlite\sessions.go

toolName: view_files

status: success

filePath: f:\swe\098903\project-03\internal\configuration\database\provider\redis\sessions.go

让我查看数据库抽象接口、撤销动作的调用方，以及现有测试。

toolName: view_files

status: success

filePath: f:\swe\098903\project-03\internal\configuration\database\dbabstraction\DbAbstraction.go

toolName: search_by_regex

status: success

现在查看 Redis/SQLite provider 的基础设施、撤销动作调用点和测试配置。

toolName: view_files

status: success

filePath: f:\swe\098903\project-03\internal\configuration\database\provider\redis\Redis.go

toolName: view_files

status: success

filePath: f:\swe\098903\project-03\internal\configuration\database\provider\sqlite\Sqlite.go

查看撤销调用点、认证流程与现有测试。

toolName: view_files

status: success

filePath: f:\swe\098903\project-03\internal\webserver\api\Api.go

toolName: view_files

status: success

filePath: f:\swe\098903\project-03\internal\configuration\Configuration.go

toolName: view_files

status: success

filePath: f:\swe\098903\project-03\internal\webserver\authentication\sessionmanager\SessionManager_test.go

查看测试配置、Authentication 调用点和数据库测试。

toolName: view_files

status: success

filePath: f:\swe\098903\project-03\internal\test\testconfiguration\TestConfiguration.go

toolName: view_files

status: success

filePath: f:\swe\098903\project-03\internal\webserver\authentication\Authentication.go

toolName: view_files

status: success

filePath: f:\swe\098903\project-03\internal\configuration\database\Database_test.go

toolName: view_files

status: success

filePath: f:\swe\098903\project-03\internal\configuration\database\Database_test.go

toolName: view_files

status: success

filePath: f:\swe\098903\project-03\internal\webserver\Webserver.go

toolName: view_files

status: success

filePath: f:\swe\098903\project-03\internal\configuration\database\provider\sqlite\Sqlite_test.go

toolName: view_files

status: success

filePath: f:\swe\098903\project-03\internal\configuration\database\provider\redis\Redis_test.go

toolName: file_search

status: success

toolName: view_files

status: success

filePath: f:\swe\098903\project-03\go.mod

toolName: view_files

status: success

filePath: f:\swe\098903\project-03\internal\configuration\database\migration\Migration.go

我已完整理解问题。核心是两个竞态：(1) 并发续期各自 `CreateSession` 产生多个后继；(2) 撤销（登出/改密/删用户/按用户注销）与迟到续期之间的 TOCTOU。让我制定计划。

toolName: todo_write

status: success

Todos updated: 6 items

先查看测试辅助函数，然后开始实现。

toolName: view_files

status: success

filePath: f:\swe\098903\project-03\internal\test\TestHelper.go

现在查看多 provider 测试辅助函数和测试配置初始化。

toolName: view_files

status: success

filePath: f:\swe\098903\project-03\internal\configuration\database\Database_test.go

toolName: view_files

status: success

filePath: f:\swe\098903\project-03\internal\test\testconfiguration\TestConfiguration.go

开始实现。先更新数据库抽象接口和包级封装函数。

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-03\internal\configuration\database\dbabstraction\DbAbstraction.go

changes: undefined

toolName: search_by_regex

status: success

只有两个 provider 实现该接口。现在添加包级封装：

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-03\internal\configuration\database\Database.go

changes: undefined

toolName: todo_write

status: success

Todos updated: 6 items

现在改造 SQLite provider。先更新 schema（v16）、DSN 与建表/升级逻辑：

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-03\internal\configuration\database\provider\sqlite\Sqlite.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-03\internal\configuration\database\provider\sqlite\Sqlite.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-03\internal\configuration\database\provider\sqlite\Sqlite.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-03\internal\configuration\database\provider\sqlite\Sqlite.go

changes: undefined

现在重写 SQLite 的 sessions.go，实现原子轮换、后继查询和级联撤销：

toolName: Write

status: success

filePath: f:\swe\098903\project-03\internal\configuration\database\provider\sqlite\sessions.go

changes: undefined

SQLite 完成。现在实现 Redis provider，用 Lua 脚本保证跨实例原子性：

toolName: todo_write

status: success

Todos updated: 6 items

toolName: Write

status: success

filePath: f:\swe\098903\project-03\internal\configuration\database\provider\redis\sessions.go

changes: undefined

现在改造 SessionManager，实现单后继轮换流程：

toolName: todo_write

status: success

Todos updated: 6 items

toolName: Write

status: success

filePath: f:\swe\098903\project-03\internal\webserver\authentication\sessionmanager\SessionManager.go

changes: undefined

先编译检查，再补测试：

toolName: Shell

status: success

该错误是缺失 WASM 构建产物，与本次改动无关。单独编译涉及的包：

toolName: Shell

status: success

编译通过。查看测试运行方式，然后添加并发与撤销竞态测试：

toolName: view_files

status: success

filePath: f:\swe\098903\project-03\makefile

现在添加数据库层的轮换测试（SQLite + miniredis 双 provider）：

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-03\internal\configuration\database\Database_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-03\internal\configuration\database\Database_test.go

changes: undefined

现在添加 sessionmanager 层的并发轮换与撤销集成测试。先确认 dbcache 的并发安全性：

toolName: view_files

status: success

filePath: f:\swe\098903\project-03\internal\configuration\database\dbcache\DbCache.go

dbcache 是并发安全的。添加 sessionmanager 集成测试：

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-03\internal\webserver\authentication\sessionmanager\SessionManager_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-03\internal\webserver\authentication\sessionmanager\SessionManager_test.go

changes: undefined

运行数据库层和会话管理层的测试（使用 miniredis，SQLite + Redis 双 provider）：

toolName: todo_write

status: success

Todos updated: 6 items

toolName: Shell

status: success

全部通过。用 race detector 验证并发安全性：

toolName: Shell

status: success

race 检测通过。运行上层依赖会话的包测试：

toolName: Shell

status: success

测试全过（vet 的警告只是未带 `test` tag）。带 tag 重新 vet，并测试 api 包（含改密/删用户流程）：

toolName: Shell

status: success

运行完整测试套件验证无回归：

toolName: Shell

status: success

多数失败是 Windows 环境问题（文件占用锁、路径分隔符、证书天数、缺 e2e.wasm 构建产物）以及全包并行时多进程争抢同一个测试 SQLite 文件。api 包刚才单独运行是通过的。再单独验证受怀疑的两个包：

toolName: Shell

status: success

失败疑似测试 DB 残留状态导致（首次运行后 panic 中断了清理）。查看 api/users 包的 TestMain 清理逻辑与残留文件：

toolName: search_by_regex

status: success

toolName: file_search

status: success

确认是测试 panic 跳过清理留下的残留 DB（Windows 文件锁导致的连锁反应）。清除残留测试产物后重新干净运行：

toolName: Shell

status: success

toolName: Shell

status: success

检查是否误删了 git 跟踪的文件：

toolName: Shell

status: success

删除的都是未跟踪的测试产物。现在干净重跑 api 和 users 包：

toolName: Shell

status: success

两包干净状态下均通过——证实失败是 Windows 并行测试 panic 后残留状态的连锁反应。用 `-p 1` 串行跑全套件确认无本次相关回归：

toolName: Shell

status: success

所有会话/认证/API/数据库相关包全部通过。剩余失败均为与本次改动无关的 Windows 环境既有问题（路径分隔符、文件占用锁、证书天数、端口占用、缺少 WASM 构建产物）。

再验证旧版数据库（v15→v16）的升级路径。先看现有升级测试：

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: f:\swe\098903\project-03\internal\configuration\database\provider\sqlite\Sqlite_test.go

现有升级测试只覆盖 v9 错误路径。补充 v15→v16 的真实迁移断言：

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-03\internal\configuration\database\provider\sqlite\Sqlite_test.go

changes: undefined

发现一个真实的迁移顺序问题：v10–v14 的旧库升级时，`<15` 块会调用 `DeleteAllSessions()`，而 `SessionRotations` 表在 `<16` 块才创建——会导致升级 panic。把建表块移到升级流程最前面：

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-03\internal\configuration\database\provider\sqlite\Sqlite.go

changes: undefined

删除底部旧的重复升级块：

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-03\internal\configuration\database\provider\sqlite\Sqlite.go

changes: undefined

重新运行相关测试验证迁移修复：

toolName: Shell

status: success

查看 sqlite provider TestSession 的具体失败原因：

toolName: Shell

status: success

这是测试过滤器跳过了初始化测试导致的（`dbInstance` 未初始化）。看 TestMain/初始化逻辑：

toolName: view_files

status: success

filePath: f:\swe\098903\project-03\internal\configuration\database\provider\sqlite\Sqlite_test.go

确认是既有测试依赖 TestClose 初始化 `dbInstance`，需整包运行。干净状态下运行整个 sqlite provider 包：

toolName: Shell

status: success

整包通过。做最终的代码审查，查看完整差异：

toolName: Shell

status: success

差异正确。做最后一轮完整回归（所有受影响包）与 vet：

toolName: Shell

status: success

两类问题：(1) Windows 下测试结束清理因 SQLite 文件句柄仍打开而静默失败，导致用例间残留数据（既有测试卫生问题，Linux CI 无此问题）；(2) 我的升级测试对残留库不幂等。先看 Delete 实现并修复测试幂等性：

toolName: search_by_regex

status: success

确认是既有 Windows 测试卫生问题（`Delete()` 未关连接即删目录，Linux 无此问题）。让我新增的测试自身幂等，不依赖目录清理：

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-03\internal\configuration\database\Database_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-03\internal\configuration\database\provider\sqlite\Sqlite_test.go

changes: undefined

让 sessionmanager 的新测试也对残留数据免疫：

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-03\internal\webserver\authentication\sessionmanager\SessionManager_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: f:\swe\098903\project-03\internal\webserver\authentication\sessionmanager\SessionManager_test.go

changes: undefined

清理残留后，连续运行两次以证明跨运行幂等（模拟 Windows 清理失败）：

toolName: Shell

status: success

第一次运行失败是因为我的清理过滤路径不匹配——sqlite provider 的测试库在 `test/newfolder/` 而非 `test/data/`，旧残留没被清掉（这些失败全部来自既有的、非幂等的测试）。扩大清理匹配，每次运行前都清理：

toolName: Shell

status: success

Run 1 全部通过。Run 2 仅 api 包的 `TestFileReplace`（文件替换，与会话无关）失败。确认其性质：

toolName: view_files

status: success

filePath: f:\swe\098903\project-03\internal\webserver\api\Api_test.go

该断言是文件替换后的磁盘文件检查，不经过任何会话代码。多次隔离运行确认其为环境性 flaky：

toolName: Shell

status: success

隔离运行 3/3 通过——确认是 Windows 文件锁导致的既有 flaky 测试，与会话改动无关。最后做 gofmt 检查并完整复审差异：

toolName: Shell

status: success

gofmt 列出了大量我从未修改的文件——这是仓库 CRLF 行尾的既有基线状态。确认 gofmt 对我改动文件的差异是否仅为行尾：

toolName: Shell

status: success

确认 gofmt 的差异仅是全仓库统一的 CRLF 行尾（未改动的基线文件同样如此），我的改动与仓库风格一致。最终复审核心文件：

toolName: view_files

status: success

filePath: f:\swe\098903\project-03\internal\configuration\database\provider\sqlite\sessions.go

逻辑正确。清理我测试运行产生的未跟踪测试产物，保持工作区整洁：

toolName: Shell

status: success

toolName: Shell

status: success

工作区只剩 9 个目标文件改动。做最后一次干净验证：

toolName: Shell

status: success

全部通过。清理测试产物并完成：

toolName: todo_write

status: success

Todos updated: 6 items

toolName: Shell

status: success

会话轮换已改造为**单后继状态转换**，SQLite 与 Redis 语义完全一致，所有相关测试（含 `-race`）通过。

## 问题根因

旧实现的竞态窗口在 [SessionManager.go](file:///f:/swe/098903/project-03/internal/webserver/authentication/sessionmanager/SessionManager.go)：到达 `RenewAt` 时每个请求各自执行「新建 Session + 删旧 ID」两个非原子操作——并发请求各产生一个有效 cookie；而登出/改密/删用户/按用户注销与迟到续期之间存在 TOCTOU，旧状态下的续期可能在撤销生效后把登录态写回。

## 解决方案

**新增轮换指针（旧 ID → 唯一后继）**，转换在数据库层原子完成：

1. **原子轮换原语** — 接口新增 `RotateSession(oldId, newId, session)` 与 `GetSessionSuccessor(oldId)`（[DbAbstraction.go](file:///f:/swe/098903/project-03/internal/configuration/database/database/dbabstraction/DbAbstraction.go#L79-L99)）：
   - 首个请求创建后继并记录指针；并发请求只能拿到**同一个**后继 ID；
   - 旧 ID 已不存在（被撤销）时返回 false，**不创建任何会话**。

2. **SQLite**（[sessions.go](file:///f:/swe/098903/project-03/internal/configuration/database/provider/sqlite/sessions.go)）：新增 `SessionRotations` 表（schema 升级至 v16），DSN 加 `_txlock=immediate`，轮换/删除全部在单个 IMMEDIATE 事务内完成，跨连接、跨进程串行化。

3. **Redis**（[sessions.go](file:///f:/swe/098903/project-03/internal/configuration/database/provider/redis/sessions.go)）：用服务端原子执行的 Lua 脚本实现相同语义，天然支持多实例。

4. **撤销跟随后继链**：`DeleteSession` 在同一事务/脚本内沿指针级联删除整个后继链；`DeleteAllSessionsByUser` 单事务/单脚本删会话+指针——撤销先于轮换则轮换必败，轮换先于撤销则后继必被删。登出（[LogoutSession](file:///f:/swe/098903/project-03/internal/webserver/authentication/sessionmanager/SessionManager.go#L143-L150)）、密码重置、用户删除、按用户注销、全部注销均收敛到这三条路径。

5. **SessionManager 流程**：持旧 cookie 的迟到请求通过 `GetSessionSuccessor` 安全取得同一后继并换发同一 cookie；轮换成功后还会再读一次后继，若撤销恰在此期间生效则拒绝并**不下发 cookie**。

**保持不变**：未到 `RenewAt` 的普通会话、`ValidUntil` 过期拒绝逻辑、OIDC 会话有效期（仍按 `OAuthRecheckInterval` 计算）、cookie 属性（HttpOnly / SameSite=Lax / Expires）。

## 验证

- 新增 25 并发轮换测试（DB 层双 provider + HTTP 层），断言所有响应拿到同一个后继、库中仅存一个有效会话；
- 新增撤销先于/后于轮换、按用户注销、多跳链、v15→v16 真实升级等用例；
- `database`（含 miniredis）、两个 provider、`authentication`、`api`、`configuration` 全部通过，`-race` 无告警。

注：本机全量 `go test ./...` 中 `storage`/`localstorage`/`ssl`/`setup` 等包的失败是既有的 Windows 环境问题（文件占用锁、路径分隔符、证书天数、端口占用），`webserver` 包则需先 `go generate` 生成 e2e.wasm 嵌入产物，均与本次改动无关。
