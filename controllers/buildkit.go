package controllers

import (
	"context"
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	v1 "kube-deploy/api/v1alpha1"

	batchv1 "k8s.io/api/batch/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	defaultBuildRegistry = "registry.registry.svc.cluster.local:5000"
	defaultPullRegistry  = "localhost:31999"
)

func EnsureBuild(ctx context.Context, c client.Client, app *v1.App) (string, bool, error) {
	log := log.FromContext(ctx).WithValues("app", app.Name, "namespace", app.Namespace)

	branch := resolveBranch(app)
	log.Info("checking latest commit", "repo", app.Spec.Repo, "branch", branch)
	commit, err := getLatestCommit(ctx, c, app)
	if err != nil {
		log.Error(err, "failed to get latest commit", "repo", app.Spec.Repo)
		return "", false, err
	}
	log.Info("got latest commit", "commit", commit)

	pullRegistry := app.Spec.Run.Registry
	if pullRegistry == "" {
		pullRegistry = defaultPullRegistry
	}

	// Already on this commit+dockerfile and healthy — verify the image is
	// actually still in the registry before trusting status. Registry cleanup
	// (or a registry pod losing storage) can erase the image out from under a
	// Ready app, leaving pods unable to restart.
	expectedImage := resolvePullImage(*app, commit)
	forceRebuild := false
	if app.Status.Commit == commit &&
		app.Status.Phase == "Ready" &&
		app.Status.Image == expectedImage &&
		app.Status.PendingCommit == "" {
		exists, checked := imageExistsInRegistry(ctx, app, imageTag(*app, commit))
		if !checked || exists {
			log.Info("already up to date, skipping build", "commit", commit, "image", app.Status.Image)
			return app.Status.Image, true, nil
		}
		log.Info("image missing from registry, forcing rebuild", "image", expectedImage)
		forceRebuild = true
	}

	// Check if any build job is currently active for this app
	var jobList batchv1.JobList
	if err := c.List(ctx, &jobList,
		client.InNamespace(app.Namespace),
		client.MatchingLabels{"kube-deploy/app": app.Name},
	); err != nil {
		return "", false, fmt.Errorf("listing build jobs: %w", err)
	}

	for _, job := range jobList.Items {
		if job.Status.Active > 0 {
			// A build is running — queue the new commit if it's different
			if commit != app.Status.Commit && commit != app.Status.PendingCommit {
				log.Info("build active, queuing new commit",
					"activeJob", job.Name,
					"queuedCommit", commit[:7],
				)
				app.Status.PendingCommit = commit
				if err := c.Status().Update(ctx, app); err != nil {
					log.Error(err, "failed to store pending commit")
				}
			} else {
				log.Info("build active, waiting for completion", "activeJob", job.Name)
			}
			return "", false, nil
		}

		// Job just finished — check if we have a pending commit to build next
		if (job.Status.Succeeded > 0 || job.Status.Failed > 0) && app.Status.PendingCommit != "" {
			log.Info("build finished, pending commit found — starting next build",
				"finishedJob", job.Name,
				"nextCommit", app.Status.PendingCommit[:7],
			)
			// Promote pending to current target
			commit = app.Status.PendingCommit
			app.Status.PendingCommit = ""
			if err := c.Status().Update(ctx, app); err != nil {
				log.Error(err, "failed to clear pending commit")
			}
			break
		}
	}

	if app.Status.Commit != commit {
		log.Info("new commit detected, triggering rebuild",
			"previous", app.Status.Commit,
			"latest", commit,
		)
	}

	pushImage := resolvePushImage(*app, commit)
	pullImage := resolvePullImage(*app, commit)
	tag := imageTag(*app, commit)
	jobName := fmt.Sprintf("%s-build-%s", app.Name, tag)
	log.Info("resolved build target", "pushImage", pushImage, "pullImage", pullImage, "job", jobName)

	// The tag encodes every build input (commit, dockerfile hash, rebuild
	// epoch) — if the registry already has it, adopt it instead of building.
	// Status drift must never cause repeat builds of an existing image.
	if !forceRebuild {
		if exists, checked := imageExistsInRegistry(ctx, app, tag); checked && exists {
			log.Info("image already in registry, adopting without rebuild", "image", pullImage)
			updateStatus(ctx, c, app, "Ready", commit, pullImage)
			return pullImage, true, nil
		}
	}

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

	// Forced rebuild with a finished job under the same name: the old job
	// completed but its image is gone — remove it so a fresh build can run.
	if forceRebuild && (job.Status.Succeeded > 0 || job.Status.Failed > 0) {
		log.Info("deleting completed job to force rebuild", "job", jobName)
		if err := c.Delete(ctx, &job, client.PropagationPolicy("Background")); err != nil {
			log.Error(err, "failed to delete stale job", "job", jobName)
		}
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
		// The new image is current — drop old tags so the registry only keeps
		// what's running. Digest-aware: never deletes a manifest the current
		// tag points to.
		if err := cleanupOldRegistryTags(ctx, app, tag); err != nil {
			log.Error(err, "old registry tag cleanup failed (best-effort)")
		}
		return pullImage, true, nil
	}

	if job.Status.Failed > 0 {
		log.Error(nil, "build job failed", "job", jobName, "failures", job.Status.Failed)
		updateStatus(ctx, c, app, "Failed", commit, "")
		// If there's a pending commit, try building that next
		if app.Status.PendingCommit != "" {
			log.Info("build failed but pending commit exists, will retry with pending",
				"pendingCommit", app.Status.PendingCommit[:7],
			)
		}
		return "", false, fmt.Errorf("build job %s failed", jobName)
	}

	log.Info("build job still running", "job", jobName, "active", job.Status.Active)
	return "", false, nil
}

func resolvePushImage(app v1.App, commit string) string {
	tag := imageTag(app, commit)
	if app.Spec.Build.Output != "" {
		return fmt.Sprintf("%s:%s", app.Spec.Build.Output, tag)
	}
	registry := app.Spec.Build.Registry
	if registry == "" {
		registry = defaultBuildRegistry
	}
	return fmt.Sprintf("%s/%s/%s:%s", registry, app.Namespace, app.Name, tag)
}

// imageTag returns a tag that captures all inputs that should trigger a new build:
// the commit prefix, an inline-dockerfile hash, and a periodic rebuild epoch.
func imageTag(app v1.App, commit string) string {
	base := commit[:7]
	suffix := ""

	if app.Spec.Build.DockerfileMode == "inline" && app.Spec.Build.Dockerfile != "" {
		h := fnv.New32a()
		h.Write([]byte(app.Spec.Build.Dockerfile))
		suffix += fmt.Sprintf("-df%x", h.Sum32())
	}

	if app.Spec.Build.NoCache {
		suffix += "-nc"
	}

	if app.Spec.Build.RebuildInterval != "" {
		if d, err := time.ParseDuration(app.Spec.Build.RebuildInterval); err == nil && d > 0 {
			epoch := time.Now().UTC().Truncate(d).Unix()
			suffix += fmt.Sprintf("-e%d", epoch)
		}
	}

	return base + suffix
}

// jobNameFromImage derives the build job name from the image currently tracked in status.
// This is the authoritative way to identify the "current" job even when the tag format evolves.
func jobNameFromImage(appName, image string) string {
	if image == "" {
		return ""
	}
	if i := strings.LastIndex(image, ":"); i >= 0 {
		return fmt.Sprintf("%s-build-%s", appName, image[i+1:])
	}
	return ""
}

func resolvePullImage(app v1.App, commit string) string {
	buildRegistry := app.Spec.Build.Registry
	if buildRegistry == "" {
		buildRegistry = defaultBuildRegistry
	}
	pullRegistry := app.Spec.Run.Registry
	if pullRegistry == "" {
		pullRegistry = defaultPullRegistry
	}
	pushImage := resolvePushImage(app, commit)
	return strings.Replace(pushImage, buildRegistry, pullRegistry, 1)
}

func updateStatus(ctx context.Context, c client.Client, app *v1.App, phase, commit, image string) {
	log := log.FromContext(ctx).WithValues("app", app.Name, "namespace", app.Namespace)
	log.Info("updating status", "phase", phase, "commit", commit, "image", image)

	app.Status.Phase = phase
	app.Status.Commit = commit
	app.Status.Image = image
	app.Status.LastUpdate = time.Now().Format(time.RFC3339)
	// Clear any prior failure message now that we have a clean phase transition.
	app.Status.Message = ""

	if err := c.Status().Update(ctx, app); err != nil {
		log.Error(err, "failed to update status", "phase", phase)
	}
}
