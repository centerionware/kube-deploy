package controllers

import (
	"context"
	"fmt"

	v1 "kube-deploy/api/v1alpha1"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func ensureBuildJob(ctx context.Context, c client.Client, app *v1.App, name string, image string) error {
	log := log.FromContext(ctx).WithValues("app", app.Name, "namespace", app.Namespace, "job", name)

	job := buildJob(app, name, image)

	log.Info("submitting build job", "image", image, "repo", app.Spec.Repo, "dockerfileMode", dockerfileMode(app))
	if err := c.Create(ctx, &job); err != nil {
		log.Error(err, "failed to create build job")
		return err
	}

	log.Info("build job submitted", "job", name)
	return nil
}

func dockerfileMode(app *v1.App) string {
	switch app.Spec.Build.DockerfileMode {
	case "generate", "inline":
		return app.Spec.Build.DockerfileMode
	default:
		return "auto"
	}
}

func buildJob(app *v1.App, name string, image string) batchv1.Job {
	jobLabels := map[string]string{
		"kube-deploy/app":       app.Name,
		"kube-deploy/namespace": app.Namespace,
	}

	// Base volumes — workspace always present
	volumes := []corev1.Volume{
		{
			Name: "workspace",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	// If a registry push secret is specified, mount it as a docker config
	// BuildKit reads ~/.docker/config.json for registry auth
	buildkitVolumeMounts := []corev1.VolumeMount{
		{Name: "workspace", MountPath: "/workspace"},
	}

	var buildkitEnv []corev1.EnvVar

	if app.Spec.Build.RegistrySecret != "" {
		volumes = append(volumes, corev1.Volume{
			Name: "registry-auth",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: app.Spec.Build.RegistrySecret,
					Items: []corev1.KeyToPath{
						{
							Key:  ".dockerconfigjson",
							Path: "config.json",
						},
					},
				},
			},
		})
		buildkitVolumeMounts = append(buildkitVolumeMounts, corev1.VolumeMount{
			Name:      "registry-auth",
			MountPath: "/root/.docker",
			ReadOnly:  true,
		})
	}

	// Build the buildctl args
	buildctlArgs := []string{
		"--addr", "tcp://buildkit-buildkit-service.buildkit.svc.cluster.local:1234",
		"build",
		"--frontend", "dockerfile.v0",
		"--local", "context=/workspace",
		"--local", "dockerfile=/workspace",
		"--opt", "filename=Dockerfile",
	}

	// Output — insecure for local registry, normal for authenticated remote
	if app.Spec.Build.RegistrySecret != "" {
		// Authenticated push — don't set insecure flag
		buildctlArgs = append(buildctlArgs,
			"--output", fmt.Sprintf("type=image,name=%s,push=true", image),
		)
	} else {
		// Local insecure registry
		buildctlArgs = append(buildctlArgs,
			"--output", fmt.Sprintf("type=image,name=%s,push=true,registry.insecure=true", image),
		)
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
					Volumes:       volumes,
					InitContainers: buildInitContainers(app),
					Containers: []corev1.Container{
						{
							Name:         "buildkit",
							Image:        "moby/buildkit:latest",
							Command:      []string{"buildctl"},
							Args:         buildctlArgs,
							Env:          buildkitEnv,
							VolumeMounts: buildkitVolumeMounts,
							Resources:    buildResources(app.Spec.Build.Resources),
						},
					},
				},
			},
		},
	}
}

func buildInitContainers(app *v1.App) []corev1.Container {
	mode := dockerfileMode(app)

	branch := resolveBranch(app)
	cloneContainer := corev1.Container{
		Name:  "git-clone",
		Image: "alpine/git",
		Command: []string{
			"sh", "-c",
			fmt.Sprintf("git clone --depth=1 --branch %s %s /workspace", branch, app.Spec.Repo),
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "workspace", MountPath: "/workspace"},
		},
		Resources: cloneResources(app.Spec.Build.Resources),
	}

	switch mode {
	case "inline":
		return []corev1.Container{
			cloneContainer,
			{
				Name:  "write-dockerfile",
				Image: "busybox",
				Command: []string{
					"sh", "-c",
					fmt.Sprintf("cat <<'DOCKERFILE' > /workspace/Dockerfile\n%s\nDOCKERFILE", app.Spec.Build.Dockerfile),
				},
				VolumeMounts: []corev1.VolumeMount{
					{Name: "workspace", MountPath: "/workspace"},
				},
			},
		}

	case "generate":
		dockerfile := generateDockerfile(*app)
		return []corev1.Container{
			cloneContainer,
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
		}

	default: // "auto"
		dockerfile := generateDockerfile(*app)
		return []corev1.Container{
			cloneContainer,
			{
				Name:  "write-dockerfile",
				Image: "busybox",
				Command: []string{
					"sh", "-c",
					fmt.Sprintf(
						`if [ ! -f /workspace/Dockerfile ]; then
  echo "No Dockerfile found, using generated one"
  cat <<'DOCKERFILE' > /workspace/Dockerfile
%s
DOCKERFILE
else
  echo "Using existing Dockerfile from repo"
fi`,
						dockerfile,
					),
				},
				VolumeMounts: []corev1.VolumeMount{
					{Name: "workspace", MountPath: "/workspace"},
				},
			},
		}
	}
}

// buildResources returns resource requirements for the buildkit container.
// Falls back to conservative defaults to prevent OOM on the node.
func buildResources(r v1.BuildResourceSpec) corev1.ResourceRequirements {
	cpuReq := "500m"
	memReq := "512Mi"
	cpuLim := "2"
	memLim := "4Gi"

	if r.CPURequest != "" {
		cpuReq = r.CPURequest
	}
	if r.MemoryRequest != "" {
		memReq = r.MemoryRequest
	}
	if r.CPULimit != "" {
		cpuLim = r.CPULimit
	}
	if r.MemoryLimit != "" {
		memLim = r.MemoryLimit
	}

	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(cpuReq),
			corev1.ResourceMemory: resource.MustParse(memReq),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(cpuLim),
			corev1.ResourceMemory: resource.MustParse(memLim),
		},
	}
}

// cloneResources returns resource requirements for the git-clone init container.
func cloneResources(r v1.BuildResourceSpec) corev1.ResourceRequirements {
	cpuReq := "100m"
	memReq := "128Mi"
	cpuLim := "500m"
	memLim := "512Mi"

	if r.CloneCPURequest != "" {
		cpuReq = r.CloneCPURequest
	}
	if r.CloneMemoryRequest != "" {
		memReq = r.CloneMemoryRequest
	}
	if r.CloneCPULimit != "" {
		cpuLim = r.CloneCPULimit
	}
	if r.CloneMemoryLimit != "" {
		memLim = r.CloneMemoryLimit
	}

	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(cpuReq),
			corev1.ResourceMemory: resource.MustParse(memReq),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(cpuLim),
			corev1.ResourceMemory: resource.MustParse(memLim),
		},
	}
}
