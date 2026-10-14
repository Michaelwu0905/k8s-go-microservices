# Ride Sharing 网约车微服务示例（Go + Kubernetes）

一个用 Go 编写的网约车示例项目，演示微服务架构在 Kubernetes 上的开发与部署。包含三个后端服务和一个地图前端，司机位置通过 **WebSocket + gRPC** 实时推送到浏览器。

## 服务架构

| 服务 | 端口 | 技术栈 | 职责 |
|------|------|--------|------|
| **api-gateway** | HTTP `:8081` | Go + gorilla/websocket | 统一入口，路由 `/services`、`/ws/drivers`、`/trip`，通过 gRPC 调用下游服务 |
| **driver-service** | HTTP `:8082` / gRPC `:9092` | Go + gRPC | 司机位置管理，按预设路线移动并流式推送 |
| **trip-service** | gRPC `:9093` | Go + gRPC | 行程创建 |
| **web** | `:3000` | Next.js 15 + React 19 + Leaflet | 地图前端（天地图底图），实时显示司机位置 |

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
