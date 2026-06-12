### locked down

```yml
apiVersion: kube-deploy.centerionware.app/v1alpha1
kind: App
metadata:
  name: holyclaude
  namespace: holyclaude
spec:
  repo: https://github.com/CoderLuii/HolyClaude

  build:
    dockerfileMode: inline
    branch: master
    registry: registry.registry.svc.cluster.local:5000
    noCache: true
    buildArgs:
      VARIANT: "full"    # Change to "slim" for smaller image
    resources:
      cpuRequest: "250m"
      memoryRequest: 512Mi
      cpuLimit: "2"
      memoryLimit: 4Gi
    dockerfile: |
      FROM node:26.2.0-bookworm-slim

      LABEL org.opencontainers.image.source=https://github.com/CoderLuii/HolyClaude

      ARG S6_OVERLAY_VERSION=3.2.3.0
      ARG TARGETARCH
      ARG VARIANT=full

      ENV DEBIAN_FRONTEND=noninteractive \
          LANG=en_US.UTF-8 \
          LC_ALL=en_US.UTF-8 \
          DISPLAY=:99 \
          DBUS_SESSION_BUS_ADDRESS=disabled: \
          CHROMIUM_FLAGS="--no-sandbox --disable-gpu --disable-dev-shm-usage" \
          CHROME_PATH=/usr/bin/chromium \
          PUPPETEER_EXECUTABLE_PATH=/usr/bin/chromium

      RUN apt-get update && apt-get install -y --no-install-recommends xz-utils curl ca-certificates && rm -rf /var/lib/apt/lists/*
      ADD https://github.com/just-containers/s6-overlay/releases/download/v${S6_OVERLAY_VERSION}/s6-overlay-noarch.tar.xz /tmp/
      RUN S6_ARCH=$(case "$TARGETARCH" in arm64) echo "aarch64";; *) echo "x86_64";; esac) && \
          curl -fsSL -o /tmp/s6-overlay-arch.tar.xz \
            "https://github.com/just-containers/s6-overlay/releases/download/v${S6_OVERLAY_VERSION}/s6-overlay-${S6_ARCH}.tar.xz" && \
          tar -C / -Jxpf /tmp/s6-overlay-noarch.tar.xz && \
          tar -C / -Jxpf /tmp/s6-overlay-arch.tar.xz && \
          rm /tmp/s6-overlay-*.tar.xz

      RUN apt-get update && apt-get install -y --no-install-recommends \
          git curl wget jq ripgrep fd-find unzip zip tree tmux fzf bat bubblewrap \
          build-essential pkg-config python3 python3-pip python3-venv \
          chromium \
          fonts-liberation2 fonts-dejavu-core fonts-noto-core fonts-noto-color-emoji \
          locales \
          strace lsof iproute2 procps htop \
          postgresql-client redis-tools sqlite3 \
          openssh-client \
          xvfb \
          imagemagick \
          sudo \
          && rm -rf /var/lib/apt/lists/*

      RUN test -x /usr/bin/bwrap && chown root:root /usr/bin/bwrap && chmod 4755 /usr/bin/bwrap

      RUN curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg \
            | dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg 2>/dev/null && \
          echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" \
            > /etc/apt/sources.list.d/github-cli.list && \
          apt-get update && apt-get install -y gh && rm -rf /var/lib/apt/lists/*

      RUN ln -sf /usr/bin/batcat /usr/local/bin/bat 2>/dev/null || true
      RUN sed -i '/en_US.UTF-8/s/^# //g' /etc/locale.gen && locale-gen

      RUN usermod -l claude -d /home/claude -m node && \
          groupmod -n claude node && \
          echo "claude ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/claude && \
          chmod 0440 /etc/sudoers.d/claude

      WORKDIR /workspace
      USER claude
      RUN curl -fsSL https://claude.ai/install.sh | bash
      USER root
      ENV PATH="/home/claude/.local/bin:${PATH}"

      RUN npm i -g \
          typescript tsx \
          pnpm \
          vite esbuild \
          eslint prettier \
          serve nodemon concurrently \
          dotenv-cli

      RUN pip install --no-cache-dir --break-system-packages \
          requests httpx beautifulsoup4 lxml \
          Pillow \
          pandas numpy \
          openpyxl python-docx \
          jinja2 pyyaml python-dotenv markdown \
          rich click tqdm \
          playwright \
          apprise

      RUN npm i -g @google/gemini-cli @openai/codex task-master-ai
      USER claude
      RUN curl -fsSL https://cursor.com/install | bash
      USER root

      # Install CloudCLI from npm registry — no vendored tarball
      RUN npm i -g @cloudcli-ai/cloudcli
      RUN touch $(npm root -g)/@cloudcli-ai/cloudcli/.env 2>/dev/null || \
          touch $(npm root -g)/@siteboon/claude-code-ui/.env 2>/dev/null || true
      # Wrapper script so s6 run script can exec claude-code-ui regardless of actual binary name
      RUN touch /usr/local/lib/node_modules/@cloudcli-ai/cloudcli/.env 2>/dev/null || \
          touch /usr/local/lib/node_modules/@siteboon/claude-code-ui/.env 2>/dev/null || true

      # Install plugins from latest HEAD — no pinned hashes
      USER claude
      RUN mkdir -p /home/claude/.claude-code-ui/plugins && \
          git clone --depth 1 https://github.com/cloudcli-ai/cloudcli-plugin-terminal.git \
            /home/claude/.claude-code-ui/plugins/web-terminal && \
          cd /home/claude/.claude-code-ui/plugins/web-terminal && \
          npm install && npm run build && \
          git clone --depth 1 https://github.com/cloudcli-ai/cloudcli-plugin-starter.git \
            /home/claude/.claude-code-ui/plugins/project-stats && \
          cd /home/claude/.claude-code-ui/plugins/project-stats && \
          npm install && npm run build && \
          git clone --depth 1 https://github.com/grostim/cloudcli-cron.git \
            /home/claude/.claude-code-ui/plugins/cloudcli-plugin-workspace-scheduled-prompts && \
          cd /home/claude/.claude-code-ui/plugins/cloudcli-plugin-workspace-scheduled-prompts && \
          npm install && npm run build && \
          echo '{"project-stats":{"name":"project-stats","source":"https://github.com/cloudcli-ai/cloudcli-plugin-starter","enabled":true},"web-terminal":{"name":"web-terminal","source":"https://github.com/cloudcli-ai/cloudcli-plugin-terminal","enabled":true},"cloudcli-plugin-workspace-scheduled-prompts":{"name":"cloudcli-plugin-workspace-scheduled-prompts","source":"https://github.com/grostim/cloudcli-cron","enabled":true}}' \
            > /home/claude/.claude-code-ui/plugins.json
      USER root

      RUN echo "${VARIANT}" > /etc/holyclaude-variant

      # All scripts and config from the repo — no modifications
      COPY scripts/entrypoint.sh /usr/local/bin/entrypoint.sh
      COPY scripts/bootstrap.sh /usr/local/bin/bootstrap.sh
      COPY scripts/notify.py /usr/local/bin/notify.py
      COPY config/settings.json /usr/local/share/holyclaude/settings.json
      COPY config/claude-memory-full.md /usr/local/share/holyclaude/claude-memory-full.md
      COPY config/claude-memory-slim.md /usr/local/share/holyclaude/claude-memory-slim.md
      RUN chmod +x /usr/local/bin/entrypoint.sh \
          /usr/local/bin/bootstrap.sh \
          /usr/local/bin/notify.py

      COPY s6-overlay/s6-rc.d/cloudcli/type /etc/s6-overlay/s6-rc.d/cloudcli/type
      COPY s6-overlay/s6-rc.d/cloudcli/run /etc/s6-overlay/s6-rc.d/cloudcli/run
      COPY s6-overlay/s6-rc.d/xvfb/type /etc/s6-overlay/s6-rc.d/xvfb/type
      COPY s6-overlay/s6-rc.d/xvfb/run /etc/s6-overlay/s6-rc.d/xvfb/run
      RUN chmod +x /etc/s6-overlay/s6-rc.d/cloudcli/run \
          /etc/s6-overlay/s6-rc.d/xvfb/run && \
          touch /etc/s6-overlay/s6-rc.d/user/contents.d/cloudcli && \
          touch /etc/s6-overlay/s6-rc.d/user/contents.d/xvfb
      RUN printf '#!/bin/sh\nexec cloudcli "$@"\n' > /usr/local/bin/claude-code-ui && \
                chmod a+x /usr/local/bin/claude-code-ui
      WORKDIR /workspace

      HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
        CMD curl -sf http://localhost:3001/ || exit 1

      ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]

  run:
    command: []
    port: 3001
    enableServiceLinks: false

    containerSecurityContext:
      runAsUser: 0
      runAsGroup: 0

    resources:
      cpuRequest: 100m
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
      netbird.io/policy: "holyclaude"
      netbird.io/sourceGroups: "ai"
  resources:
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

### sysadmin unrestricted

```yml
apiVersion: kube-deploy.centerionware.app/v1alpha1
kind: App
metadata:
  name: holyclaude
  namespace: holyclaude
spec:
  repo: https://github.com/CoderLuii/HolyClaude

  build:
    dockerfileMode: inline
    branch: master
    registry: registry.registry.svc.cluster.local:5000
    noCache: true
    buildArgs:
      VARIANT: "full"    # Change to "slim" for smaller image
    resources:
      cpuRequest: "250m"
      memoryRequest: 512Mi
      cpuLimit: "2"
      memoryLimit: 4Gi
    dockerfile: |
      FROM node:26.2.0-bookworm-slim

      LABEL org.opencontainers.image.source=https://github.com/CoderLuii/HolyClaude

      ARG S6_OVERLAY_VERSION=3.2.3.0
      ARG TARGETARCH
      ARG VARIANT=full

      ENV DEBIAN_FRONTEND=noninteractive \
          LANG=en_US.UTF-8 \
          LC_ALL=en_US.UTF-8 \
          DISPLAY=:99 \
          DBUS_SESSION_BUS_ADDRESS=disabled: \
          CHROMIUM_FLAGS="--no-sandbox --disable-gpu --disable-dev-shm-usage" \
          CHROME_PATH=/usr/bin/chromium \
          PUPPETEER_EXECUTABLE_PATH=/usr/bin/chromium

      RUN apt-get update && apt-get install -y --no-install-recommends xz-utils curl ca-certificates && rm -rf /var/lib/apt/lists/*
      ADD https://github.com/just-containers/s6-overlay/releases/download/v${S6_OVERLAY_VERSION}/s6-overlay-noarch.tar.xz /tmp/
      RUN S6_ARCH=$(case "$TARGETARCH" in arm64) echo "aarch64";; *) echo "x86_64";; esac) && \
          curl -fsSL -o /tmp/s6-overlay-arch.tar.xz \
            "https://github.com/just-containers/s6-overlay/releases/download/v${S6_OVERLAY_VERSION}/s6-overlay-${S6_ARCH}.tar.xz" && \
          tar -C / -Jxpf /tmp/s6-overlay-noarch.tar.xz && \
          tar -C / -Jxpf /tmp/s6-overlay-arch.tar.xz && \
          rm /tmp/s6-overlay-*.tar.xz

      RUN apt-get update && apt-get install -y --no-install-recommends \
          git curl wget jq ripgrep fd-find unzip zip tree tmux fzf bat bubblewrap \
          build-essential pkg-config python3 python3-pip python3-venv \
          chromium \
          fonts-liberation2 fonts-dejavu-core fonts-noto-core fonts-noto-color-emoji \
          locales \
          strace lsof iproute2 procps htop \
          postgresql-client redis-tools sqlite3 \
          openssh-client \
          xvfb \
          imagemagick \
          sudo \
          && rm -rf /var/lib/apt/lists/*

      RUN test -x /usr/bin/bwrap && chown root:root /usr/bin/bwrap && chmod 4755 /usr/bin/bwrap

      RUN curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg \
            | dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg 2>/dev/null && \
          echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" \
            > /etc/apt/sources.list.d/github-cli.list && \
          apt-get update && apt-get install -y gh && rm -rf /var/lib/apt/lists/*

      RUN ln -sf /usr/bin/batcat /usr/local/bin/bat 2>/dev/null || true
      RUN sed -i '/en_US.UTF-8/s/^# //g' /etc/locale.gen && locale-gen

      RUN usermod -l claude -d /home/claude -m node && \
          groupmod -n claude node && \
          echo "claude ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/claude && \
          chmod 0440 /etc/sudoers.d/claude

      WORKDIR /workspace
      USER claude
      RUN curl -fsSL https://claude.ai/install.sh | bash
      USER root
      ENV PATH="/home/claude/.local/bin:${PATH}"

      RUN npm i -g \
          typescript tsx \
          pnpm \
          vite esbuild \
          eslint prettier \
          serve nodemon concurrently \
          dotenv-cli

      RUN pip install --no-cache-dir --break-system-packages \
          requests httpx beautifulsoup4 lxml \
          Pillow \
          pandas numpy \
          openpyxl python-docx \
          jinja2 pyyaml python-dotenv markdown \
          rich click tqdm \
          playwright \
          apprise

      RUN npm i -g @google/gemini-cli @openai/codex task-master-ai
      USER claude
      RUN curl -fsSL https://cursor.com/install | bash
      USER root

      # Install CloudCLI from npm registry — no vendored tarball
      RUN npm i -g @cloudcli-ai/cloudcli
      RUN touch $(npm root -g)/@cloudcli-ai/cloudcli/.env 2>/dev/null || \
          touch $(npm root -g)/@siteboon/claude-code-ui/.env 2>/dev/null || true
      # Wrapper script so s6 run script can exec claude-code-ui regardless of actual binary name
      RUN touch /usr/local/lib/node_modules/@cloudcli-ai/cloudcli/.env 2>/dev/null || \
          touch /usr/local/lib/node_modules/@siteboon/claude-code-ui/.env 2>/dev/null || true

      # Install plugins from latest HEAD — no pinned hashes
      USER claude
      RUN mkdir -p /home/claude/.claude-code-ui/plugins && \
          git clone --depth 1 https://github.com/cloudcli-ai/cloudcli-plugin-terminal.git \
            /home/claude/.claude-code-ui/plugins/web-terminal && \
          cd /home/claude/.claude-code-ui/plugins/web-terminal && \
          npm install && npm run build && \
          git clone --depth 1 https://github.com/cloudcli-ai/cloudcli-plugin-starter.git \
            /home/claude/.claude-code-ui/plugins/project-stats && \
          cd /home/claude/.claude-code-ui/plugins/project-stats && \
          npm install && npm run build && \
          git clone --depth 1 https://github.com/grostim/cloudcli-cron.git \
            /home/claude/.claude-code-ui/plugins/cloudcli-plugin-workspace-scheduled-prompts && \
          cd /home/claude/.claude-code-ui/plugins/cloudcli-plugin-workspace-scheduled-prompts && \
          npm install && npm run build && \
          echo '{"project-stats":{"name":"project-stats","source":"https://github.com/cloudcli-ai/cloudcli-plugin-starter","enabled":true},"web-terminal":{"name":"web-terminal","source":"https://github.com/cloudcli-ai/cloudcli-plugin-terminal","enabled":true},"cloudcli-plugin-workspace-scheduled-prompts":{"name":"cloudcli-plugin-workspace-scheduled-prompts","source":"https://github.com/grostim/cloudcli-cron","enabled":true}}' \
            > /home/claude/.claude-code-ui/plugins.json
      USER root

      RUN echo "${VARIANT}" > /etc/holyclaude-variant

      # All scripts and config from the repo — no modifications
      COPY scripts/entrypoint.sh /usr/local/bin/entrypoint.sh
      COPY scripts/bootstrap.sh /usr/local/bin/bootstrap.sh
      COPY scripts/notify.py /usr/local/bin/notify.py
      COPY config/settings.json /usr/local/share/holyclaude/settings.json
      COPY config/claude-memory-full.md /usr/local/share/holyclaude/claude-memory-full.md
      COPY config/claude-memory-slim.md /usr/local/share/holyclaude/claude-memory-slim.md
      RUN chmod +x /usr/local/bin/entrypoint.sh \
          /usr/local/bin/bootstrap.sh \
          /usr/local/bin/notify.py

      COPY s6-overlay/s6-rc.d/cloudcli/type /etc/s6-overlay/s6-rc.d/cloudcli/type
      COPY s6-overlay/s6-rc.d/cloudcli/run /etc/s6-overlay/s6-rc.d/cloudcli/run
      COPY s6-overlay/s6-rc.d/xvfb/type /etc/s6-overlay/s6-rc.d/xvfb/type
      COPY s6-overlay/s6-rc.d/xvfb/run /etc/s6-overlay/s6-rc.d/xvfb/run
      RUN chmod +x /etc/s6-overlay/s6-rc.d/cloudcli/run \
          /etc/s6-overlay/s6-rc.d/xvfb/run && \
          touch /etc/s6-overlay/s6-rc.d/user/contents.d/cloudcli && \
          touch /etc/s6-overlay/s6-rc.d/user/contents.d/xvfb
      RUN printf '#!/bin/sh\nexec cloudcli "$@"\n' > /usr/local/bin/claude-code-ui && \
                chmod a+x /usr/local/bin/claude-code-ui
      WORKDIR /workspace

      HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
        CMD curl -sf http://localhost:3001/ || exit 1

      ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]

  run:
    command: []
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
      cpuRequest: 100m
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
      netbird.io/policy: "holyclaude"
      netbird.io/sourceGroups: "ai"

  resources:
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