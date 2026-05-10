# Flux installation:

```
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: kube-deploy
  namespace: flux-system

spec:
  interval: 1m
  url: https://github.com/centerionware/kube-deploy
  ref:
    branch: main
---
apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: kube-deploy
  namespace: flux-system

spec:
  interval: 1m

  chart:
    spec:
      chart: ./chart
      sourceRef:
        kind: GitRepository
        name: kube-deploy
        namespace: flux-system

  values:
    image:
      repository: ghcr.io/centerionware/kube-nb-qd
      tag: latest
      pullPolicy: Always
```

# Example App
```
apiVersion: kube-deploy.centerionware.app/v1alpha1
kind: App
metadata:
  name: meet
  namespace: livekit-meet
spec:
  repo: https://github.com/livekit-examples/meet

  run:
    port: 3000

  env:
    LIVEKIT_API_KEY: "devkey"
    LIVEKIT_API_SECRET: "devsecret"
    LIVEKIT_URL: "ws://livekit.livekit.svc.cluster.local:7880"

  service:
    ports:
      - port: 80
        targetPort: 3000
        protocol: TCP
    annotations:
      netbird.io/expose: "true"
      netbird.io/groups: "media"
```