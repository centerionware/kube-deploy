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
  namespace: apps

spec:
  name: meet
  repo: https://github.com/livekit-examples/meet

  service:
    name: meet
    annotations:
      netbird.io/expose: "true"
      netbird.io/groups: "media"
```