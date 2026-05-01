# Flux installation:

```
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: kube-npm-platform
  namespace: flux-system

spec:
  interval: 1m
  url: https://github.com/centerionware/kube-npm-netbird-deployer
  ref:
    branch: main
---
apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: npm-operator
  namespace: flux-system

spec:
  interval: 1m

  chart:
    spec:
      chart: ./chart
      sourceRef:
        kind: GitRepository
        name: kube-npm-platform
        namespace: flux-system

  values:
    image:
      repository: ghcr.io/centerionware/kube-nb-qd
      tag: latest
      pullPolicy: Always
```

# Example App
```
apiVersion: npm.centerionware.app/v1alpha1
kind: NpmApp
metadata:
  name: meet
  namespace: npm-deployer
spec:
  repo: https://github.com/livekit-examples/meet
  registry: registry.registry.svc.cluster.local:5000
  # build:
  #  installCmd: pnpm install
  #  buildCmd: pnpm build
  #run:
  #  command: ["pnpm", "start"]
  run:
    port: 3000
  env:
    LIVEKIT_API_KEY: "your-key"
    LIVEKIT_API_SECRET: "your-secret"
    LIVEKIT_URL: "ws://your-livekit-url"
  service:
    annotations:
      netbird.io/expose: "true"
      netbird.io/groups: "media"
```