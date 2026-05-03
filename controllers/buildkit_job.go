package controllers

import (
	"context"
	"fmt"

	v1 "kube-deploy/api/v1alpha1"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func ensureBuildJob(ctx context.Context, c client.Client, app *v1.App, name string, image string) error {
	log := log.FromContext(ctx).WithValues("app", app.Name, "namespace", app.Namespace, "job", name)

	job := buildJob(app, name, image)

	log.Info("submitting build job", "image", image, "repo", app.Spec.Repo)
	if err := c.Create(ctx, &job); err != nil {
		log.Error(err, "failed to create build job")
		return err
	}

	log.Info("build job submitted", "job", name)
	return nil
}

func buildJob(app *v1.App, name string, image string) batchv1.Job {
	dockerfile := generateDockerfile(*app)

	jobLabels := map[string]string{
		"kube-deploy/app":       app.Name,
		"kube-deploy/namespace": app.Namespace,
	}

	return batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: app.Namespace,
			Labels:    jobLabels,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: int32Ptr(1),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: jobLabels},
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
								fmt.Sprintf("git clone --depth=1 %s /workspace", app.Spec.Repo),
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "workspace", MountPath: "/workspace"},
							},
						},
						{
							Name:  "write-dockerfile",
							Image: "busybox",
							Command: []string{
								"sh", "-c",
								fmt.Sprintf("cat <<'DOCKERFILE' > /workspace/Dockerfile\n%s\nDOCKERFILE", dockerfile),
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
							Command: []string{"buildctl"},
							Args: []string{
								"--addr", "tcp://buildkit-buildkit-service.buildkit.svc.cluster.local:1234",
								"build",
								"--frontend", "dockerfile.v0",
								"--local", "context=/workspace",
								"--local", "dockerfile=/workspace",
								"--opt", "filename=Dockerfile",
								"--output", fmt.Sprintf("type=image,name=%s,push=true,registry.insecure=true", image),
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
