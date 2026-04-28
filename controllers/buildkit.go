package controllers

import (
	"context"
	"fmt"

	v1 "npm-operator/api/v1alpha1"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func resolveImage(app v1.NpmApp) string {

	reg := app.Spec.Build.Registry

	url := reg.URL
	if url == "" {
		url = "registry.registry.svc.cluster.local:5000"
	}

	repo := reg.Repository
	if repo == "" {
		repo = "apps"
	}

	return fmt.Sprintf("%s/%s/%s:latest", url, repo, app.Name)
}

func ensureBuildJob(ctx context.Context, c client.Client, app v1.NpmApp) (string, error) {

	jobName := app.Name + "-build"
	image := resolveImage(app)

	var job batchv1.Job
	err := c.Get(ctx, client.ObjectKey{Name: jobName, Namespace: app.Namespace}, &job)

	if err == nil {
		// already exists
		return image, nil
	}

	if !errors.IsNotFound(err) {
		return "", err
	}

	// ConfigMap for Dockerfile
	dockerfile := generateDockerfile(app)

	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name + "-dockerfile",
			Namespace: app.Namespace,
		},
		Data: map[string]string{
			"Dockerfile": dockerfile,
		},
	}

	_ = c.Create(ctx, &cm)

	volumes := []corev1.Volume{
		{
			Name: "workspace",
			VolumeSource: corev1.EmptyDirVolumeSource{},
		},
		{
			Name: "dockerfile",
			VolumeSource: corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: cm.Name,
				},
			},
		},
	}

	volumeMounts := []corev1.VolumeMount{
		{Name: "workspace", MountPath: "/workspace"},
		{Name: "dockerfile", MountPath: "/workspace"},
	}

	container := corev1.Container{
		Name:  "build",
		Image: "moby/buildkit:rootless",
		Command: []string{
			"buildctl-daemonless.sh",
			"build",
			"--frontend=dockerfile.v0",
			"--local=context=/workspace",
			"--local=dockerfile=/workspace",
			"--output", fmt.Sprintf("type=image,name=%s,push=true", image),
		},
		VolumeMounts: volumeMounts,
	}

	job = batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: app.Namespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers:    []corev1.Container{container},
					Volumes:       volumes,
				},
			},
		},
	}

	if err := c.Create(ctx, &job); err != nil {
		return "", err
	}

	return image, nil
}