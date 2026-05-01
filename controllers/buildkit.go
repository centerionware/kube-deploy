package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	v1 "npm-operator/api/v1alpha1"

	batchv1 "k8s.io/api/batch/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	defaultRegistry     = "registry.registry.svc.cluster.local:5000"
	defaultPullRegistry = "localhost:31999"
)

func EnsureBuild(ctx context.Context, c client.Client, app *v1.NpmApp) (string, bool, error) {
	log := log.FromContext(ctx).WithValues("npmapp", app.Name, "namespace", app.Namespace)

	log.Info("checking latest commit", "repo", app.Spec.Repo)
	commit, err := getLatestCommit(app.Spec.Repo)
	if err != nil {
		log.Error(err, "failed to get latest commit", "repo", app.Spec.Repo)
		return "", false, err
	}
	log.Info("got latest commit", "commit", commit)

	if app.Status.Commit == commit && app.Status.Phase == "Ready" {
		log.Info("already up to date, skipping build", "commit", commit, "image", app.Status.Image)
		return app.Status.Image, true, nil
	}

	pushImage := resolvePushImage(*app, commit)
	pullImage := resolvePullImage(*app, commit)
	jobName := fmt.Sprintf("%s-build-%s", app.Name, commit[:7])
	log.Info("resolved build target", "pushImage", pushImage, "pullImage", pullImage, "job", jobName)

	var job batchv1.Job
	err = c.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: jobName}, &job)
	if err != nil {
		log.Info("build job not found, creating", "job", jobName)
		if err := ensureBuildJob(ctx, c, app, jobName, pushImage); err != nil {
			log.Error(err, "failed to create build job", "job", jobName)
			return "", false, err
		}
		log.Info("build job created", "job", jobName)
		updateStatus(ctx, c, app, "Building", commit, pullImage)
		return "", false, nil
	}

	log.Info("found existing build job", "job", jobName,
		"succeeded", job.Status.Succeeded,
		"failed", job.Status.Failed,
		"active", job.Status.Active,
	)

	if job.Status.Succeeded > 0 {
		log.Info("build succeeded", "job", jobName, "pullImage", pullImage)
		updateStatus(ctx, c, app, "Ready", commit, pullImage)
		return pullImage, true, nil
	}

	if job.Status.Failed > 0 {
		log.Error(nil, "build job failed", "job", jobName, "failures", job.Status.Failed)
		updateStatus(ctx, c, app, "Failed", commit, "")
		return "", false, fmt.Errorf("build job %s failed", jobName)
	}

	log.Info("build job still running", "job", jobName, "active", job.Status.Active)
	return "", false, nil
}

// resolvePushImage returns the image ref used by buildkitd to push to the registry.
// Uses the in-cluster DNS name which buildkitd can resolve fine.
func resolvePushImage(app v1.NpmApp, commit string) string {
	if app.Spec.Build.Output != "" {
		return fmt.Sprintf("%s:%s", app.Spec.Build.Output, commit[:7])
	}
	registry := app.Spec.Registry
	if registry == "" {
		registry = defaultRegistry
	}
	return fmt.Sprintf("%s/%s:%s", registry, app.Name, commit[:7])
}

// resolvePullImage returns the image ref written into the Deployment.
// Uses localhost:31999 so containerd on any node can pull without
// needing in-cluster DNS resolution or HTTPS.
func resolvePullImage(app v1.NpmApp, commit string) string {
	pushImage := resolvePushImage(app, commit)
	// Replace the push registry host with the NodePort pull address
	registry := app.Spec.Registry
	if registry == "" {
		registry = defaultRegistry
	}
	return strings.Replace(pushImage, registry, defaultPullRegistry, 1)
}

func updateStatus(ctx context.Context, c client.Client, app *v1.NpmApp, phase, commit, image string) {
	log := log.FromContext(ctx).WithValues("npmapp", app.Name, "namespace", app.Namespace)
	log.Info("updating status", "phase", phase, "commit", commit, "image", image)

	app.Status.Phase = phase
	app.Status.Commit = commit
	app.Status.Image = image
	app.Status.LastUpdate = time.Now().Format(time.RFC3339)

	if err := c.Status().Update(ctx, app); err != nil {
		log.Error(err, "failed to update status", "phase", phase)
	}
}
