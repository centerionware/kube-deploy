# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`kube-deploy` is a Kubernetes operator (built on `controller-runtime`) that builds container images directly from git
repos (no Dockerfile required) and deploys them — collapsing the build-push-deploy pipeline into two CRDs:

- **`App`** — clone a git repo, generate or use a Dockerfile, build a multi-arch image with an in-cluster BuildKit
  daemon, push to a local registry, then create the Deployment/Service/etc. Polls the repo on `updateInterval`
  (default 1m) and rebuilds on new commits.
- **`ContainerApp`** — same runtime/service/ingress/volume/autoscaling surface as `App`, but skips the build stage and
  deploys a pre-built image directly.

Read `README.md` for the full CRD spec/field reference (it's authoritative and detailed — don't duplicate it here)
and `chart/Readme.md` for Flux/Helm install examples.

## Commands

There is no Makefile and no test suite yet (`Test coverage` is listed as "Planned" in the README — this is a known gap).

```bash
go build -o app .              # build the operator binary
go vet ./...                   # static checks
go mod tidy                    # sync go.mod/go.sum (the Dockerfile build also runs this)
```

Container build (multi-stage, scratch final image):

```bash
docker build -t kube-deploy .
```

The operator's own image is built and published by `.github/workflows/build-and-push.yaml` (multi-arch, via Buildx/QEMU,
pushed to `ghcr.io/centerionware/kube-nb-qd:latest` on every push to `main`) — using the root `Dockerfile`. Don't
confuse this with the in-cluster BuildKit pipeline described below: that one is what *this operator* runs to build
*user apps* from their git repos; this CI workflow is how *kube-deploy itself* gets built and shipped.
(`.github/workflows/sync-upstream.yml` is an unrelated job that mirrors an upstream fork — see "Use of AI Disclaimer"
in the README for background on this repo's relationship to its upstream.)

Helm chart lives in `chart/` (templates in `chart/templates/`, including the CRD definition `chart/templates/crd.yaml`
— when changing `api/v1alpha1` types, the CRD YAML in the chart must be kept in sync manually, there's no codegen step
wired up).

## Architecture

### Entry point & scheme registration
`main.go` builds a `controller-runtime` manager, registers the CRD scheme (`api/v1alpha1`) plus all the native k8s
types the operator manages (apps, core, batch, networking, autoscaling, rbac, gateway-api), and starts both
reconcilers: `AppReconciler` and `ContainerAppReconciler`.

### API types (`api/v1alpha1/`)
Hand-written types in `types.go` (no controller-gen/deepcopy-gen — `DeepCopyObject` is implemented manually with a
shallow `*out = *in` copy). `RunSpec`, `ServiceSpec`, `IngressSpec`, `GatewaySpec`, `RBACSpec`, `VolumeSpec`,
`AutoscalingSpec`, `ResourceSpec` etc. are shared between `AppSpec` and `ContainerAppSpec`. `Resources
[]json.RawMessage` lets users attach arbitrary raw Kubernetes manifests (applied via server-side apply, see below).

### Reconcile flow (`controllers/reconciler.go`, `containerapp_reconciler.go`)
Both reconcilers follow the same shape: recover from panics (so a bad CR can't crash the worker), apply a
per-reconcile timeout, manage a finalizer for cleanup-on-delete, then drive the CR through phases. `App` additionally
runs a build phase before runtime. Errors from build/runtime steps are treated as non-fatal — they're logged and the
item is requeued with backoff rather than returned as reconcile errors, so one broken App doesn't block the queue.

### Build pipeline (App only)
- `git.go` — uses `go-git` (in-memory, no shell-out to `git`) to resolve the latest commit on the target branch
  (`resolveBranch`, default `main`), with auth resolved from an optional `gitSecret` (HTTPS user/pass or SSH key).
- `dockerfile.go` — generates a Dockerfile from `build.installCmd`/`buildCmd`/`baseImage`/`run.command` when the repo
  doesn't provide one, or when `dockerfileMode: generate`. `dockerfileMode: inline` uses `build.dockerfile` verbatim.
- `buildkit.go` (`EnsureBuild`) — the core state machine: computes the expected image tag from commit+app, checks
  `app.Status` to decide whether a rebuild is needed, lists active build Jobs for this app (label
  `kube-deploy/app=<name>`), and either waits, queues the new commit as `Status.PendingCommit`, or starts a new build.
  Only one build Job runs per App at a time; new commits that land mid-build are queued and picked up when the
  current Job finishes.
- `buildkit_job.go` — constructs the actual Kubernetes `Job` that clones the repo and runs BuildKit
  (`buildctl`) to build and push the image.
- `utils.go` — small shared helpers used across the build/runtime code (`int32Ptr`/`int64Ptr`/`boolPtr`, `buildEnv`,
  `must` (resource.Quantity parsing), `nullIfEmpty`).

Image registries: BuildKit pushes to the in-cluster DNS name (`registry.registry.svc.cluster.local:5000` by default),
while the kubelet/containerd pulls via a NodePort (`localhost:31999` by default) since containerd can't resolve
cluster-internal DNS. `EnsureBuild`/`resolvePullImage` compute both addresses; this split is central to how the
registry integration works — see the README's "How it works" section for the full rationale.

### Runtime (shared by App and ContainerApp)
- `runtime.go` (`EnsureRuntime`) — builds the desired Deployment/Service/etc. from `RunSpec`/`ServiceSpec` (resources,
  health checks, security contexts, `EnableServiceLinks`, image pull secrets, host network, ...).
- `volumes.go`, `autoscaling.go`, `ingress.go`, `gateway.go`, `serviceaccount.go` — translate the corresponding spec
  sections (`VolumeSpec`, `AutoscalingSpec` → HPA, `IngressSpec`, `GatewaySpec` → Gateway API `HTTPRoute`, `RBACSpec`
  → ServiceAccount/Role/ClusterRole/bindings) into the matching Kubernetes objects.
- `resources.go` (`EnsureResources`) — applies arbitrary raw manifests from `spec.resources` via server-side apply
  (idempotent), labeling them `kube-deploy/app`/`kube-deploy/namespace` for ownership so they can be found and deleted
  on cleanup.
- `upsert.go` — generic create-or-update helpers; updates only happen when operator-owned fields actually changed
  (image, replicas, pod template, etc.), to avoid fighting other controllers/HPA over fields it doesn't own.

### Cleanup (`cleanup.go`, `registry_cleanup.go`)
On CR deletion (driven by the finalizer in the reconcilers), `cleanupRuntime` deletes everything the operator created
(Deployment, Service, Ingress/HTTPRoute, HPA, PVCs, RBAC, raw resources via the ownership label), and
`registry_cleanup.go` removes pushed images from the in-cluster registry.

### Conventions to follow when extending
- Everything the operator creates is labeled `kube-deploy/app=<name>` (and often `kube-deploy/namespace=<ns>`) so it
  can be listed/cleaned up later — keep new resource types consistent with this.
- Reconcile loop errors should generally be swallowed into a `ctrl.Result{RequeueAfter: ...}` rather than returned, to
  match the "one bad CR shouldn't block others" pattern already established.
- `RunSpec`/`ServiceSpec`/etc. are shared between `App` and `ContainerApp` — changes to runtime behavior usually
  belong in the shared helpers (`runtime.go`, `volumes.go`, etc.), not duplicated per-reconciler.
