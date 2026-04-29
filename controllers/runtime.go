package controllers

import (
	"context"

	v1 "npm-operator/api/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

func EnsureRuntime(ctx context.Context, c client.Client, app *v1.NpmApp, image string) error {

	labels := map[string]string{"app": app.Name}

	deploy := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    "app",
							Image:   image,
							Command: app.Spec.Run.Command,
							Args:    app.Spec.Run.Args,
							Env:     buildEnv(app.Spec.Env),
							Ports: []corev1.ContainerPort{
								{ContainerPort: int32(app.Spec.Run.Port)},
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    must("500m"),
									corev1.ResourceMemory: must("512Mi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    must("50m"),
									corev1.ResourceMemory: must("64Mi"),
								},
							},
						},
					},
				},
			},
		},
	}

	_ = upsertDeployment(ctx, c, deploy)

	svc := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        app.Name,
			Namespace:   app.Namespace,
			Annotations: app.Spec.Service.Annotations,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{
				{Port: int32(app.Spec.Run.Port)},
			},
		},
	}

	return upsertService(ctx, c, svc)
}