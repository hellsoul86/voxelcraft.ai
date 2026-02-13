# World Package Architecture

本文档描述 `internal/sim/world` 的当前架构与边界，不包含历史演进信息。

## 1. Runtime Model

- 单 world = 单 goroutine 权威模拟循环
- 网络/WS 线程只做入队与出队，不直接改世界状态
- 所有状态写入在 world loop 内完成，保证确定性
- 当前世界语义为 2D：所有写入动作必须 `y==0`

入口文件：
- `world.go`：构造与依赖装配
- `runtime_loop.go`：`Run` / `StepOnce`
- `runtime_step.go`：单 tick 调度主流程
- `runtime_api_core.go`：核心运行时 API
- `runtime_api_admin.go`：admin 请求 API（snapshot/reset）
- `runtime_api_transfer*.go`：跨 world 迁移与查询 API（transfer / agent_pos / events / org_meta）
- `debug_api.go`：仅测试/调试辅助 API

## 2. Authoritative Tick Order

`stepInternal` 的顺序是确定性契约：
1. 季节与 reset notice
2. join / leave
3. transfer out / transfer in
4. maintenance
5. 应用 ACT（按接收顺序）
6. 系统执行（固定顺序）：
   - movement
   - work
   - conveyor
   - environment
   - laws
   - director
   - contracts
   - fun
7. 生成并发送 OBS
8. observer stream
9. digest / snapshot
10. metrics 更新与 tick 自增

## 3. Package Boundaries

### 3.1 world façade（`internal/sim/world`）

职责：
- 持有并管理全部可变状态
- 统一事件投递与审计写入
- 调用 feature/logic/io/policy 子包
- 对外暴露 `World` API
- 仅保留“编排壳 + 适配器壳”：
  - `*_facade.go`：系统调度与状态落点
    - 例如：`session_facade.go`、`contracts_facade.go`、`conveyor_facade.go`、`survival_facade.go`、`entities_runtime_facade.go`、`entities_items_runtime_facade.go`
    - director/fun/season 已按域拆分：`director_runtime_facade.go`、`director_events_facade.go`、`season_facade.go`、`fun_facade.go`
  - `instants_adapter_factory.go`：instant env 的 world->featurectx 构造入口
  - `tasks_adapter_factory.go`：任务执行环境适配器（work/claim/movement 分域）
  - `runtime_api_*.go`：核心/admin/transfer 三类 API 面
  - `movement_tasks_facade.go` / `work_tasks_facade.go`：任务系统 façade（movement/work）
  - `tasks_dispatch.go`：task req 分发表（handler 映射）
  - `terrain_runtime_facade.go`：2D 地形/方块运行时语义 helper

实现约束说明（第一性原则）：
- 需要直接操作 `World` 私有状态或定义 `World` 方法的代码必须留在根包（Go 语言限制：不能在外包为非本包类型定义方法）
- 因此根包保留少量 `*_facade.go`/`*_dispatch.go`/`*_adapter_factory.go` 作为编排壳
- 可纯化的算法、规则、编解码与业务流程已下沉到 `feature/*`、`logic/*`、`io/*`、`policy/*`

### 3.2 Kernel（`internal/sim/world/kernel/model`）

职责：
- 核心模型与通用方法
- 例如：`Agent`、`LandClaim`、`Organization`、`Contract`、`ItemEntity`、`Vec3i`

### 3.3 Terrain（`internal/sim/world/terrain/*`）

职责：
- `terrain/store`：chunk 存储与访问
- `terrain/gen`：确定性 worldgen

### 3.4 Feature（`internal/sim/world/feature/*`）

按业务域拆分：
- `admin`：admin 请求处理
  - `admin/debug`：debug API 的纯状态变更逻辑
- `session`：join/attach/welcome/catalog/memory/chat
- `transfer`：跨 world agent/org/event cursor 迁移
  - `transfer/runtime/world_handlers.go`：transfer in/out 的核心处理与 world map 变更规则（world 仅保留通道与状态挂接）
- `movement`：移动任务请求与执行辅助
  - `movement/runtime/system.go`：移动系统主循环已下沉（world 仅提供 env 适配）
- `work`：采集/放置/合成/熔炼/蓝图任务
  - `work/runtime/pull.go`：蓝图自动拉料候选筛选与扣料流程
- `economy`：交易、估值、税、库存原语
- `contracts`：合约生命周期、验收、结算、信誉联动
- `governance`：claim、law、org、maintenance、权限
  - `governance/runtime`：land/org membership 判定与 claim maintenance 执行流程
  - `governance/laws/runtime/lifecycle.go`：法律状态机与投票结算主循环（world 仅注入 hooks）
  - `governance/laws/runtime/apply.go`：法律模板应用到 land 的纯执行器
- `director`：事件调度、资源刷点、fun 统计
- `observer`：OBS 视图投影与 observer stream
  - `observer/stream/client_runtime.go`：observer chunk/voxel 客户端状态机（world 仅提供回调）
  - `observer/stream/messages.go`：observer 协议消息编码与构造
- `survival`：环境压力、复活逻辑
- `entities`：掉落物实体规则
  - `entities/runtime`：container/sign/conveyor/switch 运行时元数据规则
- `conveyor`：物流带运行规则
- `persistence`：snapshot/digest 编排辅助
  - `persistence/snapshot/config_patch.go`：snapshot 配置/运行态补丁构建（world 只做赋值编排）

### 3.5 Pure Helper Layers

- `logic/*`：纯算法与纯计算
- `io/*`：纯编解码
- `policy/rules`：纯规则判定
- `featurectx/*`：feature 调用 world façade 的小接口适配层
  - 例如：`featurectx/workexec`、`featurectx/workrequest`、`featurectx/claimrequest`、`featurectx/movement`
  - instant 分域适配：`featurectx/instants/{session,economy,contracts,conveyor,governance,observerposting}`

## 4. Dependency Rules

1. `world` 可以 import `feature/*`、`logic/*`、`io/*`、`policy/*`、`kernel/*`、`terrain/*`
2. `feature/*` 与 `featurectx/*` 不 import `internal/sim/world`
3. `feature/*` 通过 DTO + 回调接口与 world façade 协作
4. `logic/*` / `io/*` / `policy/*` 保持无状态纯函数
5. 禁止循环依赖（`go list ./internal/sim/world/...` 必须通过）

## 5. Determinism Invariants

- Tick 顺序不可变
- OBS 字段与排序规则不可漂移
- voxel 扫描与编码顺序不可漂移
- state digest 写入顺序不可漂移
- 写世界动作必须满足 2D 约束（`y==0`）

## 6. Testing Strategy

- `internal/sim/world`：集成白盒（需要 world 私有状态）
- `internal/sim/worldtest`：黑盒回归（只用公开 API + Debug 辅助）
- `feature/*`、`logic/*`、`io/*`、`policy/*`：纯单测
- 顶层 `world` 保留 integration 测试，子包继续放纯逻辑单测；以“编排在 world，算法在子包”为分层准则
- `instant` handlers 已按域拆分在 root（`instants_*_handlers.go`），dispatch 独立在 `instants_dispatch.go`

验证命令：

```bash
go test ./internal/sim/world/... 
go test ./internal/sim/multiworld ./cmd/server
go test ./...
scripts/release_gate.sh --skip-race
scripts/release_gate.sh --with-agent --skip-race --agent-dir /home/vscode/projects/voxelcraft.agent --scenario multiworld_mine_trade_govern --count 50 --duration 60
```
