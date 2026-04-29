package controllers

import (
	v1 "npm-operator/api/v1alpha1"
)

func generateDockerfile(app v1.NpmApp) string {

	build := app.Spec.Build

	base := "node:20-alpine"
	if build.BaseImage != "" {
		base = build.BaseImage
	}

	install := "pnpm install"
	if build.InstallCommand != "" {
		install = build.InstallCommand
	}

	buildCmd := "pnpm build"
	if build.BuildCommand != "" {
		buildCmd = build.BuildCommand
	}

	return `
FROM ` + base + `
WORKDIR /app
COPY . .
RUN ` + install + `
RUN ` + buildCmd + `
CMD ` + formatCmd(app.Spec.Run.Command) + `
`
}