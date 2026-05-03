package controllers

import (
	"strings"

	v1 "kube-deploy/api/v1alpha1"
)

func generateDockerfile(app v1.App) string {
	base := "node:20-alpine"
	if app.Spec.Build.BaseImage != "" {
		base = app.Spec.Build.BaseImage
	}

	install := "npm install --legacy-peer-deps"
	if app.Spec.Build.InstallCmd != "" {
		install = app.Spec.Build.InstallCmd
	}

	build := "npm run build"
	if app.Spec.Build.BuildCmd != "" {
		build = app.Spec.Build.BuildCmd
	}

	runCmd := formatCmd(app.Spec.Run.Command, app.Spec.Run.Args)
	if runCmd == "" {
		runCmd = `["npm","start"]`
	}

	return strings.TrimSpace(`
FROM ` + base + `
WORKDIR /app

COPY . .

RUN ` + install + `
RUN ` + build + `

EXPOSE 3000

CMD ` + runCmd + `
`)
}

func formatCmd(cmd []string, args []string) string {
	full := append(cmd, args...)
	if len(full) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("[")
	for i, c := range full {
		b.WriteString(`"` + c + `"`)
		if i < len(full)-1 {
			b.WriteString(",")
		}
	}
	b.WriteString("]")
	return b.String()
}
