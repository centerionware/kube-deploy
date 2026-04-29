package controllers

import (
	"strings"

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

	cmd := formatCmd(app.Spec.Run.Command)
	if cmd == "" {
		cmd = `["node","server.js"]`
	}

	return strings.TrimSpace(`
FROM ` + base + `
WORKDIR /app
COPY . .

RUN ` + install + `
RUN ` + build + `

CMD ` + cmd + `
`)
}

// ---------------- LOCAL HELPER (ONLY USED HERE) ----------------

func formatCmd(cmd []string) string {
	if len(cmd) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("[")

	for i, c := range cmd {
		b.WriteString(`"` + c + `"`)
		if i < len(cmd)-1 {
			b.WriteString(",")
		}
	}

	b.WriteString("]")
	return b.String()
}