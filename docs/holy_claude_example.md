```yml
# ==============================================================================
# HolyClaude â€” Level 1: Minimal (run as root inside container, no extra caps)
# Try this first. May break features that need kernel capabilities.
# ==============================================================================
apiVersion: kube-deploy.centerionware.app/v1alpha1
kind: ContainerApp
metadata:
  name: holyclaude
  namespace: holyclaude
spec:
  image: coderluii/holyclaude:latest

  run:
    port: 3001
    enableServiceLinks: false

    containerSecurityContext:
      runAsUser: 0
      runAsGroup: 0
      allowPrivilegeEscalation: false

    resources:
      cpuRequest: 500m
      memoryRequest: 512Mi
      cpuLimit: "2"
      memoryLimit: 4Gi

    volumes:
      - name: claude-data
        mountPath: /home/claude/.claude
        pvc:
          claimName: claude-data
      - name: workspace
        mountPath: /workspace
        pvc:
          claimName: workspace

  env:
    TZ: "UTC"
    NODE_OPTIONS: "--max-old-space-size=4096"
    PUID: "1000"
    PGID: "1000"

  service:
    ports:
      - port: 80
        targetPort: 3001
        protocol: TCP
    annotations:
      netbird.io/expose: "true"
      netbird.io/groups: "ai"

  resources:
    - apiVersion: v1
      kind: Namespace
      metadata:
        name: holyclaude

    - apiVersion: v1
      kind: PersistentVolumeClaim
      metadata:
        name: claude-data
        namespace: holyclaude
      spec:
        accessModes:
          - ReadWriteOnce
        storageClassName: local-path
        resources:
          requests:
            storage: 5Gi

    - apiVersion: v1
      kind: PersistentVolumeClaim
      metadata:
        name: workspace
        namespace: holyclaude
      spec:
        accessModes:
          - ReadWriteOnce
        storageClassName: local-path
        resources:
          requests:
            storage: 10Gi
```
```yml
---
# ==============================================================================
# HolyClaude â€” Level 2: With SYS_PTRACE (needed for subprocess tracking)
# ==============================================================================
apiVersion: kube-deploy.centerionware.app/v1alpha1
kind: ContainerApp
metadata:
  name: holyclaude
  namespace: holyclaude
spec:
  image: coderluii/holyclaude:latest

  run:
    port: 3001
    enableServiceLinks: false

    containerSecurityContext:
      runAsUser: 0
      runAsGroup: 0
      capabilities:
        add: ["SYS_PTRACE"]

    resources:
      cpuRequest: 500m
      memoryRequest: 512Mi
      cpuLimit: "2"
      memoryLimit: 4Gi

    volumes:
      - name: claude-data
        mountPath: /home/claude/.claude
        pvc:
          claimName: claude-data
      - name: workspace
        mountPath: /workspace
        pvc:
          claimName: workspace

  env:
    TZ: "UTC"
    NODE_OPTIONS: "--max-old-space-size=4096"
    PUID: "1000"
    PGID: "1000"

  service:
    ports:
      - port: 80
        targetPort: 3001
        protocol: TCP
    annotations:
      netbird.io/expose: "true"
      netbird.io/groups: "ai"

  resources:
    - apiVersion: v1
      kind: Namespace
      metadata:
        name: holyclaude

    - apiVersion: v1
      kind: PersistentVolumeClaim
      metadata:
        name: claude-data
        namespace: holyclaude
      spec:
        accessModes:
          - ReadWriteOnce
        storageClassName: local-path
        resources:
          requests:
            storage: 5Gi

    - apiVersion: v1
      kind: PersistentVolumeClaim
      metadata:
        name: workspace
        namespace: holyclaude
      spec:
        accessModes:
          - ReadWriteOnce
        storageClassName: local-path
        resources:
          requests:
            storage: 10Gi
```
```yml
---
# ==============================================================================
# HolyClaude â€” Level 3: Full (matches docker-compose exactly)
# SYS_ADMIN + SYS_PTRACE + seccomp unconfined. Use only on trusted networks.
# ==============================================================================
apiVersion: kube-deploy.centerionware.app/v1alpha1
kind: ContainerApp
metadata:
  name: holyclaude
  namespace: holyclaude
spec:
  image: coderluii/holyclaude:latest

  run:
    port: 3001
    enableServiceLinks: false

    containerSecurityContext:
      runAsUser: 0
      runAsGroup: 0
      capabilities:
        add: ["SYS_ADMIN", "SYS_PTRACE"]
      seccompProfile:
        type: Unconfined

    resources:
      cpuRequest: 500m
      memoryRequest: 512Mi
      cpuLimit: "2"
      memoryLimit: 4Gi

    volumes:
      - name: claude-data
        mountPath: /home/claude/.claude
        pvc:
          claimName: claude-data
      - name: workspace
        mountPath: /workspace
        pvc:
          claimName: workspace

  env:
    TZ: "UTC"
    NODE_OPTIONS: "--max-old-space-size=4096"
    PUID: "1000"
    PGID: "1000"

  service:
    ports:
      - port: 80
        targetPort: 3001
        protocol: TCP
    annotations:
      netbird.io/expose: "true"
      netbird.io/groups: "ai"

  resources:
    - apiVersion: v1
      kind: Namespace
      metadata:
        name: holyclaude

    - apiVersion: v1
      kind: PersistentVolumeClaim
      metadata:
        name: claude-data
        namespace: holyclaude
      spec:
        accessModes:
          - ReadWriteOnce
        storageClassName: local-path
        resources:
          requests:
            storage: 5Gi

    - apiVersion: v1
      kind: PersistentVolumeClaim
      metadata:
        name: workspace
        namespace: holyclaude
      spec:
        accessModes:
          - ReadWriteOnce
        storageClassName: local-path
        resources:
          requests:
            storage: 10Gi
```