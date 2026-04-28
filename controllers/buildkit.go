package controllers

import (
	"context"
	"fmt"

	v1 "npm-operator/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ensureBuildJob(ctx context.Context, c client.Client, app v1.NpmApp, image string, commit string) (bool, error) {

	jobName := app.Name + "-" + commit[:7]

	var pod corev1.Pod
	err := c.Get(ctx, client.ObjectKey{
		Name:      jobName,
		Namespace: app.Namespace,
	}, &pod)

	if err == nil {
		return pod.Status.Phase == corev1.PodSucceeded, nil
	}

	if !errors.IsNotFound(err) {
		return false, err
	}

	dockerfile := generateDockerfile(app)

	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName + "-dockerfile",
			Namespace: app.Namespace,
		},
		Data: map[string]string{
			"Dockerfile": dockerfile,
		},
	}

	_ = c.Create(ctx, &cm)

	pod = corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: app.Namespace,
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,

			InitContainers: []corev1.Container{
				{
					Name:  "clone",
					Image: "alpine/git",
					Command: []string{
						"sh", "-c",
						fmt.Sprintf("git clone %s /workspace && cd /workspace && git checkout %s",
							app.Spec.Repo, commit),
					},
					VolumeMounts: []corev1.VolumeMount{
						{Name: "ws", MountPath: "/workspace"},
					},
				},
			},

			Containers: []corev1.Container{
				{
					Name:  "build",
					Image: "moby/buildkit:rootless",
					Command: []string{
						"buildctl-daemonless.sh",
						"build",
						"--frontend=dockerfile.v0",
						"--local=context=/workspace",
						"--local=dockerfile=/workspace",
						"--output",
						fmt.Sprintf("type=image,name=%s,push=true", image),
					},
					VolumeMounts: []corev1.VolumeMount{
						{Name: "ws", MountPath: "/workspace"},
					},
				},
			},

			Volumes: []corev1.Volume{
				{
					Name: "ws",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
		},
	}

	return false, c.Create(ctx, &pod)
}