package controllers

import (
	"context"
	"fmt"
	"time"

	v1 "npm-operator/api/v1alpha1"

	batchv1 "k8s.io/api/batch/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func EnsureBuild(ctx context.Context, c client.Client, app *v1.NpmApp) (string, bool, error) {

	commit, err := getLatestCommit(app.Spec.Repo)
	if err != nil {
		return "", false, err
	}

	if app.Status.Commit == commit && app.Status.Phase == "Ready" {
		return app.Status.Image, true, nil
	}

	image := resolveImage(*app, commit)
	jobName := fmt.Sprintf("%s-build-%s", app.Name, commit[:7])

	var job batchv1.Job
	err = c.Get(ctx, client.ObjectKey{
		Namespace: app.Namespace,
		Name:      jobName,
	}, &job)

	if err != nil {
		if err := ensureBuildJob(ctx, c, app, jobName, image); err != nil {
			return "", false, err
		}

		updateStatus(ctx, c, app, "Building", commit, image)
		return "", false, nil
	}

	if job.Status.Succeeded > 0 {
		updateStatus(ctx, c, app, "Ready", commit, image)
		return image, true, nil
	}

	if job.Status.Failed > 0 {
		updateStatus(ctx, c, app, "Failed", commit, "")
		return "", false, fmt.Errorf("build failed")
	}

	return "", false, nil
}

func resolveImage(app v1.NpmApp, commit string) string {
	base := app.Spec.Build.Output
	if base == "" {
		base = "registry.local/" + app.Name
	}
	return fmt.Sprintf("%s:%s", base, commit[:7])
}

func updateStatus(ctx context.Context, c client.Client, app *v1.NpmApp, phase, commit, image string) {
	app.Status.Phase = phase
	app.Status.Commit = commit
	app.Status.Image = image
	app.Status.LastUpdate = time.Now().Format(time.RFC3339)
	_ = c.Status().Update(ctx, app)
}