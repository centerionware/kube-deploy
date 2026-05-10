# kube-deploy

A lightweight Kubernetes operator that builds and deploys applications directly from git repositories — no Dockerfiles, no Helm charts, no manual image management required.

Think of it as a self-hosted Vercel or Cloudflare Workers, purpose-built for k3s clusters.

---
### project specific thoughts


i realized today that the ingress, half of the volume things, the httproute, can be simply removed because of the generic resources.

i think immediately and before widespread use this must happen so things are simplified.

im leaving them in for now because i cant be arsed myself to do it and fix the examples.


---

## Why

Kubernetes has a rich ecosystem: `kubectl`, Helm, Kustomize, Flux, ArgoCD. But all of these tools assume you already have a container image. Getting from source code to a running deployment still requires maintaining Dockerfiles, build pipelines, and chart repositories — all of which drift, go stale, and break.

The problem is worse than it sounds:

- Helm charts lag behind upstream releases and frequently go unmaintained
- Many projects — especially AI-adjacent ones — ship no Dockerfile at all
- Pre-built images are often x86-only, leaving ARM and RISC-V users to fend for themselves
- Build infrastructure (CI runners, registries, artifact stores) is expensive to operate and fragile to maintain

kube-deploy collapses the entire build-push-deploy pipeline into a single CRD. Point it at a git repository, and it handles the rest: clones the source, generates a Dockerfile if one isn't present, builds a multi-arch image using an in-cluster BuildKit daemon, pushes to a local registry, and creates the Deployment and Service. It polls for new commits and rebuilds automatically.

---

## How it works

```
App CR applied
  └── operator clones repo (go-git, no binary required)
        └── Dockerfile generated (or repo's own used)
              └── BuildKit daemon builds & pushes image
                    └── Image pushed to in-cluster registry
                          └── Deployment + Service created/updated
                                └── Ingress or HTTPRoute created (optional)
```

**Registry note:** The registry runs inside Kubernetes and is reachable from BuildKit via in-cluster DNS. However, containerd (which pulls images onto nodes) runs outside the cluster network and cannot resolve `*.svc.cluster.local`. The solution is a NodePort service on the registry — all nodes can reach it via `localhost:<nodeport>` regardless of which node the registry pod is actually running on. kube-deploy handles both addresses automatically: BuildKit pushes to the DNS name, the Deployment pulls from `localhost:31999`.

---

## CRDs

### `App` — build from source and deploy

```yaml
apiVersion: kube-deploy.centerionware.app/v1alpha1
kind: App
metadata:
  name: meet
  namespace: apps
spec:
  repo: https://github.com/livekit-examples/meet

  # How often to poll git for new commits (default: 1m)
  updateInterval: 5m

  build:
    installCmd: pnpm install   # default: npm install --legacy-peer-deps
    buildCmd: pnpm build       # default: npm run build
    baseImage: node:20-alpine  # default: node:20-alpine
    registry: registry.registry.svc.cluster.local:5000

    # Optional: git auth for private repos
    # gitSecret: my-git-secret  # k8s Secret with username+password or ssh-privatekey

  run:
    command: ["pnpm", "start"]
    port: 3000
    replicas: 1
    registry: localhost:31999

    # Optional resource overrides
    resources:
      cpuRequest: 50m
      memoryRequest: 128Mi
      cpuLimit: 500m
      memoryLimit: 512Mi

    # Optional HTTP health check (falls back to TCP if omitted)
    healthCheck:
      path: /healthz

    # Optional volumes
    volumes:
      - name: data
        mountPath: /data
        size: 5Gi
        storageClass: local-path

    # Optional autoscaling
    autoscaling:
      enabled: true
      minReplicas: 1
      maxReplicas: 5
      cpuTarget: 80

  env:
    NODE_ENV: production
    LIVEKIT_URL: ws://livekit.internal

  # Full Kubernetes Service override
  service:
    type: ClusterIP
    annotations:
      netbird.io/expose: "true"
      netbird.io/groups: "media"

  # Optional: Ingress (use either ingress or gateway, not both)
  ingress:
    enabled: true
    host: meet.example.com
    className: nginx
    tlsSecret: meet-tls

  # Optional: Gateway API HTTPRoute
  # gateway:
  #   enabled: true
  #   gatewayRef:
  #     name: main-gateway
  #   hostnames:
  #     - meet.example.com
```

### `ContainerApp` — deploy a pre-built image

For when you already have an image and just need it deployed and managed. Supports the same runtime, service, ingress, volume, and autoscaling surface as `App` — just skips the build stage.

```yaml
apiVersion: kube-deploy.centerionware.app/v1alpha1
kind: ContainerApp
metadata:
  name: nginx
  namespace: apps
spec:
  image: localhost:31999/my-namespace/myapp:abc1234

  run:
    port: 80

  service:
    annotations:
      netbird.io/expose: "true"
```

---

## Private git repositories

Create a Kubernetes Secret with your credentials and reference it in the `App` spec:

**HTTPS / token:**
```bash
kubectl create secret generic my-git-secret -n apps \
  --from-literal=username=myuser \
  --from-literal=password=ghp_yourtoken
```

**SSH:**
```bash
kubectl create secret generic my-git-secret -n apps \
  --from-file=ssh-privatekey=~/.ssh/id_ed25519
```

```yaml
spec:
  repo: git@github.com:org/private-repo.git
  build:
    gitSecret: my-git-secret
```

---

## Infrastructure requirements

kube-deploy expects the following to already be running in the cluster:

| Component | Namespace | Purpose |
|-----------|-----------|---------|
| BuildKit daemon | `buildkit` | Builds container images |
| Docker registry | `registry` | Stores built images |

Reference manifests for both are in `docs/`.

The registry service should be exposed as a NodePort on `31999` so nodes can pull images:

```bash
kubectl patch svc registry -n registry \
  -p '{"spec":{"type":"NodePort","ports":[{"port":5000,"targetPort":5000,"nodePort":31999}]}}'
```

---

## Current status

| Feature | Status |
|---------|--------|
| Git polling (public repos) | ✅ Working |
| Git auth (HTTPS + SSH) | ✅ Implemented |
| Dockerfile generation | ✅ Working |
| BuildKit integration | ✅ Working |
| Image push to local registry | ✅ Working |
| Deployment creation | ✅ Working |
| Service creation | ✅ Working |
| Automatic rebuild on git change | ✅ Working |
| Cleanup on CRD deletion | ✅ Implemented |
| Build job GC | ✅ Implemented |
| Namespace-scoped image names | ✅ Implemented |
| Health checks | ✅ Implemented |
| Resource limits | ✅ Implemented |
| Volume / PVC management | ✅ Implemented |
| Autoscaling (HPA) | ✅ Implemented |
| Ingress generation | ✅ Implemented |
| Gateway API (HTTPRoute) | ✅ Implemented |
| Full service override | ✅ Implemented |
| ContainerApp CRD | ✅ Implemented |
| Registry image cleanup on delete | ✅ Implemented |
| Registry auth | ✅ Implemented |
| Canary / rollback | 🔲 Planned |
| Test coverage | 🔲 Planned |

---

## Todo

- [ ] Write full test coverage (unit + integration) for all controllers
- [ ] Registry push authentication (`build.registrySecret`)
- [ ] Canary deployments and rollback support
- [ ] Multi-branch tracking (deploy specific branches per CR)
- [ ] Webhook-triggered reconcile (skip polling, react to push events)


---

## Supported languages

kube-deploy defaults to Node.js but works with any language by overriding the build commands and base image:

**Python:**
```yaml
build:
  baseImage: python:3.12-slim
  installCmd: pip install -r requirements.txt
  buildCmd: echo ok
run:
  command: ["python", "app.py"]
  port: 8000
```

**Go:**
```yaml
build:
  baseImage: golang:1.22-alpine
  installCmd: go mod download
  buildCmd: go build -o app .
run:
  command: ["/app/app"]
  port: 8080
```

**Any project with its own Dockerfile:** just push the Dockerfile to the repo root and kube-deploy will use it as-is.

## Dockerfile Overrides

```
# Default — uses repo Dockerfile if present, generates one if not
build: {}

# Always generate — ignores any Dockerfile in the repo
build:
  dockerfileMode: generate

# Inline — your own complete Dockerfile in the CR
build:
  dockerfileMode: inline
  dockerfile: |
    FROM node:20-alpine
    WORKDIR /app
    COPY . .
    RUN npm ci
    CMD ["node", "server.js"]
```
## Service Port Ranges

Kubernetes doesn't naturally support port ranges, but sometimes it may be useful to do so (eg: livekit)

```
service:
  ports:
    - port: 7880
      targetPort: 7880
      protocol: TCP
    - port: 7881
      targetPort: 7881
      protocol: TCP
  portRanges:
    - start: 50000
      end: 60000
      protocol: UDP
  annotations:
    netbird.io/expose: "true"
    netbird.io/groups: "media"
```

## RBAC

```
rbac:
  roles:
    - name: livekit-configmap-reader
      rules:
        - apiGroups: [""]
          resources: ["configmaps"]
          verbs: ["get", "list", "watch"]
  clusterRoles:
    - name: livekit-node-reader
      rules:
        - apiGroups: [""]
          resources: ["nodes"]
          verbs: ["get", "list"]
  clusterRoleBindings:
    - "view"  # bind existing ClusterRole
```

This creates: a ServiceAccount named livekit, a Role livekit-configmap-reader + RoleBinding, a ClusterRole kube-deploy-livekit-livekit-node-reader + ClusterRoleBinding, and binds the existing view ClusterRole — all cleaned up on deletion.

## Registry Authentication

### Push (build stage)

Create a `kubernetes.io/dockerconfigjson` secret in the same namespace as the App:

```bash
kubectl create secret docker-registry my-registry-push \
  -n my-namespace \
  --docker-server=ghcr.io \
  --docker-username=myuser \
  --docker-password=mytoken
```

Reference it in the build spec:

```yaml
spec:
  build:
    registry: ghcr.io/myorg
    registrySecret: my-registry-push
```

### Pull (runtime stage)

Same secret type, referenced in the run spec. Supports multiple registries:

```yaml
spec:
  run:
    imagePullSecrets:
      - my-registry-pull
      - another-registry-secret
```

The local insecure registry (`localhost:31999`) requires no secret — `registry.insecure=true` is set automatically when `build.registrySecret` is not specified.

## Generic literally any resource available to the cluster

```
spec:
  resources:
    - apiVersion: networking.k8s.io/v1
      kind: NetworkPolicy
      metadata:
        name: allow-ingress
      spec:
        podSelector:
          matchLabels:
            app: myapp
        ingress:
          - {}
    - apiVersion: monitoring.coreos.com/v1
      kind: ServiceMonitor
      metadata:
        name: myapp-metrics
      spec:
        selector:
          matchLabels:
            app: myapp
        endpoints:
          - port: metrics
```

Server-side apply means it's idempotent — safe to reconcile repeatedly. The operator owns them via the kube-deploy/app label and deletes them when the App is removed.


## Use of AI Disclaimer

ChatGPT was originally used to generate the skeleton of this project. Claude Sonnet 4.6 was used to get it into its current state. 

AI will be used to write everything up until an acceptably working version that supports all the planned features is available. Then it may be used to write patches if and when issues arise. 

Currently I am developing this solo, and would prefer to have a team of brilliant humans to maintain this long term, but for now, it is what it is.

## Developers

centerionware
