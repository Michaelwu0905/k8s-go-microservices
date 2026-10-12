# 迁移计划：本地部署从 minikube 切换到 k3s + UTM 环境

## Context（背景）

当前项目的本地开发流程是 `tilt up`（Tiltfile 编译 Go 二进制 → Docker 构建镜像 → 部署到 minikube → port-forward 8081）。目标是把这套开发流程整体迁移到用户已有的 **k3s + 3 台 UTM Ubuntu 虚拟机** 集群，替换掉 minikube，同时保留 Tilt 的热更新开发体验。

## 关键发现（已探明的事实）

| 项 | 值 | 影响 |
|----|----|------|
| Mac 架构 | **arm64**（Apple Silicon） | Tiltfile 里 `GOARCH=amd64` 与目标环境不符，需改为 `arm64` |
| k3s 节点架构 | 3 台均为 **arm64** Ubuntu 26.04（k3s-master `192.168.64.4`、worker-1 `192.168.64.2`、worker-2 `192.168.64.5`） | 镜像需为 arm64 |
| 容器运行时 | **containerd**（无 docker CLI） | 不能在节点上直接 `docker run`，registry 要用 k8s 方式部署 |
| Docker Desktop | 当前未运行 | Tilt 的 `docker_build` 依赖它，需先启动 |
| kubeconfig | `~/.kube/k3s-utm.yaml`，context 名 `default`（namespace `learn`）；minikube 已把 current-context 改成 `minikube` | 需先切回 `default` |
| 镜像引用 | dev k8s yaml 里写死 `ride-sharing/api-gateway` 等，无 registry 前缀 | 需要本地 registry + Tilt `default_registry()`，让节点能拉镜像 |
| 服务发现 | Go 代码里默认 `driver-service:9092` / `trip-service:9093`（集群内 DNS） | 无需改动，k3s 内 DNS 直接可用 |
| 端口访问 | api-gateway 是 `LoadBalancer`，Tilt 已配 `port_forwards=8081` | k3s 自带 ServiceLB 会分配 EXTERNAL-IP，且 localhost:8081 由 Tilt 转发，无需改 yaml |

## Approach（推荐方案）

**保留 Tilt，加一个集群内本地 registry，并修正架构参数。** 核心三件事：

1. **修架构**：Tiltfile 里三处 `GOARCH=amd64` → `GOARCH=arm64`（Mac 与 3 台 VM 都是 arm64）。
2. **建本地 registry（in-cluster）**：在 k3s-master 上以 `hostNetwork: true` 的 pod 跑 `registry:2`，绑定 `192.168.64.4:5000`；三台节点的 containerd 通过 `registries.yaml` 将其标记为 insecure mirror；Mac 的 Docker Desktop 配置 insecure registry 以 http push。
3. **Tiltfile 接入 registry**：顶部加 `default_registry('192.168.64.4:5000')`，并加 `allow_k8s_contexts('default')` 防止误操作到 minikube。Tilt 会自动把镜像名注入 k8s yaml，**无需手改镜像名**。

> 决策已定：采用**方案 A（集群内 registry + http）**；应用统一部署到新建命名空间 **`ride-sharing`**。

## Files to modify

- `Tiltfile` — 修改 3 处 `GOARCH`、加 `default_registry()` 与 `allow_k8s_contexts()`
- `infra/development/k8s/registry.yaml`（**新增**）— registry pod + 配置
- 各 VM 上的 `/etc/rancher/k3s/registries.yaml`（**集群侧，不进仓库**）— insecure mirror 配置
- Mac 的 Docker Desktop 配置（`~/.docker/daemon.json` 或 GUI）— insecure registry
- （可选）`README.md` — 补充 k3s 本地开发的说明
- `infra/development/k8s/{api-gateway,driver-service,trip-service}-deployment.yaml` — 三个文件的 Deployment 与 Service 的 `metadata` 加 `namespace: ride-sharing`

**无需改动**：Go 源码（集群内 DNS 短名 `driver-service:9092` / `trip-service:9093` 在 `ride-sharing` 命名空间内仍可用）、`web/`（默认已指向 `localhost:8081`）、`skaffold.yaml`/`cloudbuild.yaml`（生产/废弃，不动）。

## Steps（实施清单）

### 阶段 0：切回 k3s context + 建命名空间
- [x] `kubectl config use-context default`
- [x] 验证 `kubectl get nodes` 看到 3 个 Ready 节点
- [x] `kubectl create namespace ride-sharing`
- [x] `kubectl config set-context default --namespace=ride-sharing`（让 `kubectl get pods` 默认看该命名空间）

### 阶段 1：部署本地 registry（集群内）
- [x] 新增 `infra/development/k8s/registry.yaml`：
  - `Namespace: registry`
  - `Deployment registry`：`image: registry:2`、`hostNetwork: true`、`containerPort: 5000`、`nodeSelector` 固定到 `k3s-master`、加 control-plane 污点 toleration
- [x] `kubectl apply -f infra/development/k8s/registry.yaml`（registry 是基础设施，**一次性手动 apply，不进 Tiltfile**）
- [x] 三台节点各写 `/etc/rancher/k3s/registries.yaml`：
  ```yaml
  mirrors:
    "192.168.64.4:5000":
      endpoint:
        - "http://192.168.64.4:5000"
  ```
- [x] 重启 k3s 服务：master `sudo systemctl restart k3s`，worker `sudo systemctl restart k3s-agent`
- [x] 验证 `kubectl get pods -n registry` registry 就绪；节点上 `curl http://192.168.64.4:5000/v2/` 通

### 阶段 2：Mac 侧 Docker + Tiltfile
- [x] 启动 Docker Desktop
- [x] 配置 insecure registry `192.168.64.4:5000`（`~/.docker/daemon.json` 加 `"insecure-registries": ["192.168.64.4:5000"]` 或 GUI），重启 Docker
- [x] 编辑 `Tiltfile`：
  - 顶部加 `default_registry('192.168.64.4:5000')` 和 `allow_k8s_contexts('default')`
  - 三处 `GOARCH=amd64` → `GOARCH=arm64`
- [x] 验证 Mac 能 push：`docker pull alpine && docker tag alpine 192.168.64.4:5000/test && docker push 192.168.64.4:5000/test`

### 阶段 3：tilt up 与端到端验证
- [x] `tilt up`，观察 3 个服务编译 → 构建 → 推镜像 → 部署成功
- [x] `kubectl get pods -n ride-sharing` 确认 Running
- [x] `curl http://localhost:8081/services` 返回服务列表
- [x] WebSocket 验证 `ws://localhost:8081/ws/drivers` 有司机位置流（可 `wscat` 或起 web）
- [x] `kubectl scale deployment driver-service --replicas=3`，观察副本分散到不同 worker 节点、round-robin
- [x] 起 web：`cd web && npm run dev`，浏览器 localhost:3000 看到地图与司机

## Verification（验收标准）

- `kubectl get nodes`：3 节点 Ready，且 driver-service 的 3 副本能调度到不同节点（真实多节点效果）
- `curl http://localhost:8081/services` 与 `ws://localhost:8081/ws/drivers` 均可用
- 修改任一 Go 源码后 Tilt 自动热更新（live_update）仍然生效
- 不依赖 minikube：`minikube status` 不参与，删除 minikube context 也不影响

## 已确认的决策

1. **Registry 方案**：采用「集群内 registry + http」（方案 A）。
2. **命名空间**：应用统一部署到新建命名空间 `ride-sharing`；registry 单独放 `registry` 命名空间（基础设施，一次性手动 apply）。
