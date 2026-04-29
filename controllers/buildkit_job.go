package controllers

import (
	"fmt"

	v1 "npm-operator/api/v1alpha1"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ensureBuildJob(ctx context.Context, c client.Client, app *v1.NpmApp, name string, image string) error {
	job := buildJob(app, name, image)
	return c.Create(ctx, &job)
}

func buildJob(app *v1.NpmApp, name string, image string) batchv1.Job {

	dockerfile := generateDockerfile(*app)

	return batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: app.Namespace,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: int32Ptr(1),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,

					Volumes: []corev1.Volume{
						{
							Name: "workspace",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},

					InitContainers: []corev1.Container{
						{
							Name:  "git-clone",
							Image: "alpine/git",
							Command: []string{
								"sh", "-c",
								fmt.Sprintf("git clone %s /workspace", app.Spec.Repo),
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "workspace", MountPath: "/workspace"},
							},
						},
						{
							Name:  "dockerfile",
							Image: "busybox",
							Command: []string{
								"sh", "-c",
								fmt.Sprintf("cat <<'EOF' > /workspace/Dockerfile\n%s\nEOF", dockerfile),
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "workspace", MountPath: "/workspace"},
							},
						},
					},

					Containers: []corev1.Container{
						{
							Name:    "buildkit",
							Image:   "moby/buildkit:latest",
							Command: []string{"buildctl-daemonless.sh"},
							Args: []string{
								"build",
								"--frontend", "dockerfile.v0",
								"--local", "context=/workspace",
								"--local", "dockerfile=/workspace",
								"--opt", "filename=Dockerfile",
								"--output", fmt.Sprintf("type=image,name=%s,push=true", image),
							},
							Env: []corev1.EnvVar{
								{
									Name:  "BUILDKITD_FLAGS",
									Value: "--oci-worker-no-process-sandbox",
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "workspace", MountPath: "/workspace"},
							},
						},
					},
				},
			},
		},
	}
}