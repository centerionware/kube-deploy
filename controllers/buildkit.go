package controllers

import (
	"fmt"
	v1 "npm-operator/api/v1alpha1"
)

func generateDockerfile(app v1.NpmApp) string {

	base := "node:20-alpine"
	if app.Spec.Build.BaseImage != "" {
		base = app.Spec.Build.BaseImage
	}

	install := "pnpm install"
	if app.Spec.Build.InstallCmd != "" {
		install = app.Spec.Build.InstallCmd
	}

	build := "pnpm build"
	if app.Spec.Build.BuildCmd != "" {
		build = app.Spec.Build.BuildCmd
	}

	return fmt.Sprintf(`
FROM %s
WORKDIR /app
COPY . .
RUN %s
RUN %s
CMD %s
`,
		base,
		install,
		build,
		formatCmd(app.Spec.Run.Command),
	)
}

func formatCmd(cmd []string) string {
	if len(cmd) == 0 {
		return `["node","server.js"]`
	}

	out := "["
	for i, c := range cmd {
		out += fmt.Sprintf("\"%s\"", c)
		if i < len(cmd)-1 {
			out += ","
		}
	}
	out += "]"
	return out
}