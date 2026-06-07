```yml
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