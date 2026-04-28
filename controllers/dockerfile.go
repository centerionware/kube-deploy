package controllers

import (
	"fmt"
	v1 "npm-operator/api/v1alpha1"
)

func generateDockerfile(app v1.NpmApp) string {

	base := app.Spec.Build.BaseImage
	if base == "" {
		base = "node:20-alpine"
	}

	install := app.Spec.Build.InstallCommand
	if install == "" {
		install = "pnpm install"
	}

	build := app.Spec.Build.BuildCommand
	if build == "" {
		build = "pnpm build"
	}

	return fmt.Sprintf(`
FROM %s

WORKDIR /app

RUN apk add --no-cache git
RUN npm install -g pnpm

RUN git clone %s .

RUN %s
RUN %s

EXPOSE %d

CMD ["pnpm", "start"]
`, base, app.Spec.Repo, install, build, resolvePort(app))
}