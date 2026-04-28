package controllers

import (
	"bytes"
	"os/exec"
	v1 "npm-operator/api/v1alpha1"
)

func getGitSHA(app v1.NpmApp) (string, error) {

	cmd := exec.Command("git", "ls-remote", app.Spec.Repo, "HEAD")

	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return "", err
	}

	// format: <sha>\tHEAD
	parts := bytes.Split(out.Bytes(), []byte("\t"))
	return string(parts[0]), nil
}