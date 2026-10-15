# Ride Sharing 网约车微服务示例（Go + Kubernetes）

一个用 Go 编写的网约车示例项目，演示微服务架构在 Kubernetes 上的开发与部署。包含三个后端服务和一个地图前端，司机位置通过 **WebSocket + gRPC** 实时推送到浏览器。

## 服务架构

| 服务 | 端口 | 技术栈 | 职责 |
|------|------|--------|------|
| **api-gateway** | HTTP `:8081` | Go + gorilla/websocket | 统一入口，路由 `/services`、`/ws/drivers`、`/trip`，通过 gRPC 调用下游服务 |
| **driver-service** | HTTP `:8082` / gRPC `:9092` | Go + gRPC | 持久化司机位置与在线状态，流式推送附近车辆 |
| **trip-service** | gRPC `:9093` | Go + gRPC + pgx | 持久化订单、状态机、实时订单通知 |
| **web** | `:3000` | Next.js 15 + React 19 + Leaflet | 地图前端（天地图底图），实时显示司机位置 |
| **postgres** | `:5432` | PostgreSQL 16 + PVC | 订单持久化、并发约束、LISTEN/NOTIFY |

**服务间通信**：`api-gateway` 通过集群内 DNS 短名 `driver-service:9092`、`trip-service:9093` 调用下游 gRPC 服务（地址由环境变量 `DRIVER_SERVICE_URL` / `TRIP_SERVICE_URL` 覆盖）。

## 项目结构

```
k8s-go-microservices/
├── services/                  # 微服务源码
│   ├── api-gateway/           # API 网关（HTTP/WebSocket + gRPC 客户端）
│   ├── driver-service/        # 司机服务（HTTP + gRPC）
│   └── trip-service/          # 行程服务（gRPC）
├── shared/                    # 跨服务共享代码
│   ├── proto/                 # protoc 生成的 gRPC 桩代码（*.pb.go）
│   └── types/                 # 共享类型
├── proto/                     # Protobuf 源文件（.proto）
├── infra/
│   ├── development/           # 本地开发
│   │   ├── docker/            # 开发 Dockerfile（含 Windows 构建脚本）
│   │   └── k8s/               # 开发清单 + registry.yaml（本地镜像仓库）
│   └── production/            # 生产（Google Cloud 示例）
│       ├── docker/
│       └── k8s/
├── web/                       # Next.js 前端
│   ├── src/                   # 页面、组件、hooks
│   └── .env.local             # 环境变量（天地图 key，不入 git）
├── docs/                      # 历史文档
├── Tiltfile                   # Tilt 本地开发编排
├── Makefile                   # protoc 代码生成
├── cloudbuild.yaml            # Google Cloud Build（生产 CI/CD）
├── go.mod / go.sum
└── README.md
```

## 本地开发环境（k3s + UTM 虚拟机）

本项目的本地开发使用 **k3s 集群（3 台 UTM Ubuntu 虚拟机）+ Tilt**，替代传统的 minikube 方案。相比 minikube，多节点集群能真实演练副本调度、跨节点负载均衡。

### 环境要求

| 组件 | 用途 |
|------|------|
| k3s 集群（3 节点，arm64） | Kubernetes 运行时（containerd） |
| Docker Desktop | 本地镜像构建与推送 |
| Tilt | 开发循环编排（编译 → 构建 → 推镜像 → 部署） |
| Go 1.23+ | 编译 Go 服务 |
| Node.js 20+ | 前端开发 |
| 天地图 API Key | 地图底图（免费申请，见下文） |

### 集群拓扑

| 节点 | IP | 角色 |
|------|-----|------|
| k3s-master | `192.168.64.4` | control-plane |
| k3s-worker-1 | `192.168.64.2` | worker |
| k3s-worker-2 | `192.168.64.5` | worker |

### 一次性初始化（新环境只需做一次）

1. **切换 kubeconfig 并创建命名空间**

   ```bash
   kubectl config use-context default
   kubectl create namespace ride-sharing
   kubectl config set-context default --namespace=ride-sharing
   ```

2. **部署集群内本地镜像仓库**（基础设施，一次性 apply，不进 Tiltfile）

   ```bash
   kubectl apply -f infra/development/k8s/registry.yaml
   ```

   仓库以 `hostNetwork` 方式跑在 `k3s-master` 的 `192.168.64.4:5000`。

3. **三台节点配置 insecure registry**（每台节点都要做）

   创建 `/etc/rancher/k3s/registries.yaml`：

   ```yaml
   mirrors:
     "192.168.64.4:5000":
       endpoint:
         - "http://192.168.64.4:5000"
   ```

   然后重启 k3s 使配置生效：

   ```bash
   # master 节点
   sudo systemctl restart k3s
   # worker 节点
   sudo systemctl restart k3s-agent
   ```

4. **Mac 上配置 Docker Desktop insecure registry**

   编辑 `~/.docker/daemon.json`：

   ```json
   {
     "insecure-registries": ["192.168.64.4:5000"]
   }
   ```

   重启 Docker Desktop 生效。

5. **配置前端天地图 Key**

   在 `web/.env.local`（该文件已被 git 忽略）中写入：

   ```
   NEXT_PUBLIC_TIANDITU_KEY=你的天地图key
   ```

   Key 申请：访问 [天地图控制台](https://console.tianditu.gov.cn/api/key) → 创建应用 → 应用类型选「浏览器端」→ 复制 Key。个人开发者免费。

### 日常启动

```bash
# 终端 1：启动后端开发循环（自动编译、构建、推镜像、部署，并转发 localhost:8081）
tilt up

# 终端 2：启动前端
cd web && npm run dev
```

浏览器打开 **http://localhost:3000**，即可看到北京地图上移动的司机标记。

### 常用命令

```bash
# 查看服务状态
kubectl get pods -n ride-sharing

# 查看 api-gateway 服务（ServiceLB 会分配节点 IP）
kubectl get svc -n ride-sharing

# 直接通过节点 IP 访问网关（无需端口转发）
curl http://192.168.64.4:8081/services

# 扩容司机服务，观察副本分散到不同节点
kubectl scale deployment driver-service --replicas=3 -n ride-sharing

# 重新生成 gRPC 代码（修改 .proto 后）
make generate-proto
```

### 监控

```bash
kubectl get pods -n ride-sharing
kubectl logs -n ride-sharing deployment/api-gateway
```

## 技术要点

- **热更新**：Tiltfile 通过 `docker_build_with_restart` + `live_update`，修改 Go 源码后自动重新编译并同步到运行中的容器。
- **架构适配**：Mac 与 k3s 节点均为 arm64，Tiltfile 中编译参数已设为 `GOOS=linux GOARCH=arm64`。
- **镜像仓库**：Tilt 构建的镜像推送到集群内 `192.168.64.4:5000`，节点 containerd 经 `registries.yaml` 配置为 insecure 拉取。
- **坐标系**：天地图使用 CGCS2000 坐标系（与 WGS-84 差异厘米级），司机经纬度与 geohash 无需坐标转换。
- **负载均衡**：`driver-service` 使用 ClusterIP 服务，扩容后请求在副本间分发。

## 生产部署（Google Cloud，参考）

生产部署流程保留在 `cloudbuild.yaml` 与 `infra/production/` 中（Google Cloud Build + GKE + Artifact Registry），详见 [旧版 README](docs/README.minikube.md) 的 Deployment 章节。

## 历史文档

- [docs/README.minikube.md](docs/README.minikube.md) — 原始 README（minikube 方案 + Skaffold，已废弃）

## 持久化订单与司机工作台

订单保存到 PostgreSQL，司机端与乘客端通过 SSE 接收实时订单快照。
后端链路为 `PostgreSQL LISTEN/NOTIFY → trip-service gRPC stream → api-gateway SSE → 浏览器`。
每次连接先发送数据库最新快照，断线自动重连；每 15 秒补发快照，适用于多个网关和订单服务副本。

### 数据库初始化

开发集群使用 PostgreSQL 16、StatefulSet 和 1Gi 的 `local-path` PVC。
为避免虚拟机直接拉取公共镜像失败，首次使用先把官方镜像推入现有本地仓库：

```bash
docker pull postgres:16
docker tag postgres:16 192.168.64.4:5000/postgres:16
docker push 192.168.64.4:5000/postgres:16
tilt up
```

Tilt 自动部署 `infra/development/k8s/postgres.yaml`，并把 Secret 中的 `DATABASE_URL` 注入行程服务。
行程服务启动时执行内嵌的幂等建表脚本；现有内存订单没有历史数据可迁移。
开发凭据仅供此本地集群使用。生产环境需自行配置 `postgres-credentials` Secret 的 `DATABASE_URL`、备份和数据库高可用。
PVC 可保留 Pod 重建后的数据，但 `local-path` 绑定单个节点，并不提供跨节点数据库容灾。
订单时间戳由数据库生成；请保持三台 UTM 虚拟机的系统时间同步，避免暂停/恢复虚拟机后显示错误时间。

### 双端操作

1. `cd web && npm run dev`，打开 `http://localhost:3000`。
2. 在地图点击目的地，然后点击“确认叫车”。选点本身不会写入订单。
3. 另开浏览器页面 `http://localhost:3000/driver`，选择 `driver-0` 等演示身份。
4. 点击“接受订单”；乘客端自动显示接驾状态。
5. 司机依次点击“已到达上车点” → “乘客已上车，开始行程” → “到达目的地，结束行程”。
6. 刷新任一页面，当前订单从数据库恢复。乘客可在开始行程前取消订单。

状态机：`PENDING → ACCEPTED → ARRIVED → IN_TRIP → COMPLETED`；前三种状态可转为 `CANCELLED`。
行锁和数据库唯一索引保证一单只能有一个司机、司机和乘客各自只有一张进行中的订单；重复状态操作不会重复推进。

演示身份不是登录认证：乘客 ID 保存在浏览器 localStorage，司机身份由页面选择。接口检查身份与订单关联，但尚不能防止伪造 ID；仅用于当前开发环境。
司机端支持设备定位或明确标注的演示行驶；位置与在线状态保存在 PostgreSQL，停止定位会下线。

### HTTP 接口

| 接口 | 参数与行为 |
|---|---|
| `POST /trip` | `rider_id`、`pickup`、`destination`；确认下单并返回完整订单 |
| `GET /trip/{id}` | 返回订单、路线、司机、时间戳和版本 |
| `GET /trips?rider_id=…` | 乘客最近订单，进行中订单优先，最多 100 条 |
| `GET /trips?driver_id=…` | 待接订单与该司机订单，最多 100 条 |
| `POST /trip/accept` | `trip_id`、`driver_id`；司机原子接单 |
| `PATCH /trip/{id}/status` | 数字 `status` 与 `driver_id` 或 `rider_id` |
| `GET /events/trips` | SSE；指定一个 `trip_id`、`rider_id` 或 `driver_id` 过滤条件 |

状态数字：1 等待接单、2 已接单、3 已到达、4 行程中、5 已完成、6 已取消。
冲突返回 HTTP 409，身份不匹配返回 403，订单不存在返回 404。
前端地址可通过 `NEXT_PUBLIC_API_URL` 和 `NEXT_PUBLIC_WEBSOCKET_URL` 配置。
路线服务默认使用 OSRM 公共服务，可通过行程服务的 `OSRM_URL` 覆盖；请求超时为 10 秒。

### 验证

```bash
go test ./...
cd web
npx tsc --noEmit
npm run lint
```

数据库集成测试需连接可创建临时 schema 的 PostgreSQL；测试只操作随机生成的 `trip_test_*` schema，结束后清理，不删除业务表：

```bash
kubectl port-forward -n ride-sharing svc/postgres 15432:5432
# 另一个终端，使用开发集群凭据
TEST_DATABASE_URL='postgres://ride_sharing:ride-sharing-local-dev@127.0.0.1:15432/ride_sharing?sslmode=disable' go test -race ./services/trip-service -v
```

测试覆盖状态跳转、操作身份、并发抢单、司机重复接单、取消与接单竞争、跨服务实例通知，以及重新连接数据库后的订单恢复。


## 司机定位与接驾导航

司机端 `http://localhost:3000/driver` 现在提供两种位置来源：

- **演示行驶**：在北京设置初始位置。接单后沿 OSRM 接驾路线每秒推进并上报位置；上车后沿目的地路线继续行驶。到达、上车和结束仍由司机手动确认。
- **使用设备定位**：通过浏览器 `watchPosition` 获取坐标与精度，持续上报。浏览器需要定位权限和安全上下文（localhost 或 HTTPS）；地图底图使用天地图。设备定位本身可能受操作系统、浏览器及位置提供商的精度和坐标体系影响。

开启任一模式会把司机设为在线；停止定位会下线。单纯打开司机页面不会自动获取设备位置。
GPS 模式每 5 秒补报有效定位，超过 20 秒未收到新定位时停止补报；服务端把超过 30 秒未更新的位置标为过期。
关闭页面或网络中断后，过期机制也会自动隐藏未接单的车辆。

乘客地图只展示上车点 5 公里内有有效位置的在线司机；已分配司机始终保留在地图中，过期或离线时明确提示。
接单后橙色路线表示司机到乘客上车点的接驾路线；上车后蓝色路线表示前往目的地的导航路线。
双方可看到剩余距离和预计时长，车辆标记平滑移动；路线每 10 秒检查刷新，移动不足 80 米时复用当前路线。
OSRM 预计时间不包含实时交通信息。定位 WebSocket 会在断线后自动重连。

| 新接口 | 行为 |
|---|---|
| `GET /drivers/{id}` | 最近位置、有效在线状态、来源、精度和上报时间 |
| `PUT /drivers/{id}/availability` | `{ "online": true/false }`，控制司机在线状态 |
| `PUT /drivers/{id}/location` | `{ "location": { "latitude": …, "longitude": … }, "source": "gps"或"demo", "accuracy": … }` |
| `GET /trip/{id}/navigation` | 从已分配司机的有效位置规划接驾或目的地路线，返回路线及阶段 |

位置表由司机服务启动时自动创建；司机服务现在也需要 `DATABASE_URL`，开发清单与 Tilt 已包含该配置。
位置数据跨服务副本共享。`/health` 会检查数据库连接。

前端路线推进算法测试：

```bash
node --experimental-strip-types --test web/tests/geo.test.mjs
```

### UTM 虚拟机时间同步

三台虚拟机原来的 Ubuntu 授时域名被本机代理解析到 fake-IP 地址，chrony 没有持续收到有效时间样本，造成暂停/恢复后的多天偏差。
本次修复使用 [Cloudflare 官方公布的 NTP IP](https://developers.cloudflare.com/time-services/ntp/usage/) 绕过 fake-IP DNS，并通过 chrony 保持同步。

仓库脚本可在已有 SSH 别名和免密 sudo 的环境中重复执行：

```bash
./scripts/sync-vm-clocks.sh
# 或仅修复指定节点
./scripts/sync-vm-clocks.sh k3s-worker-1
```

脚本备份 `/etc/chrony/sources.d/ubuntu-ntp-pools.sources`，写入 `ride-sharing.sources` 与 `conf.d/99-utm-clock.conf`，
启用并重启 chrony，配置 `makestep 1 -1` 以处理后续暂停/恢复造成的大幅偏差。
使用 `chronyc -n sources` 查看 `^*` 已选授时源，使用 `timedatectl show -p NTPSynchronized` 验证同步。
该配置不会回填旧订单的历史时间戳。
