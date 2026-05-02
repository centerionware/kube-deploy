package controllers

import (
	"context"

	v1 "npm-operator/api/v1alpha1"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func EnsureHPA(ctx context.Context, c client.Client, app *v1.NpmApp) error {
	log := log.FromContext(ctx).WithValues("npmapp", app.Name, "namespace", app.Namespace)

	if app.Spec.Run.Autoscaling == nil || !app.Spec.Run.Autoscaling.Enabled {
		// Remove HPA if it exists and autoscaling is disabled
		var existing autoscalingv2.HorizontalPodAutoscaler
		if err := c.Get(ctx, client.ObjectKey{Name: app.Name, Namespace: app.Namespace}, &existing); err == nil {
			log.Info("removing HPA (disabled)")
			return c.Delete(ctx, &existing)
		}
		return nil
	}

	as := app.Spec.Run.Autoscaling
	minReplicas := int32(1)
	if as.MinReplicas > 0 {
		minReplicas = int32(as.MinReplicas)
	}
	maxReplicas := int32(5)
	if as.MaxReplicas > 0 {
		maxReplicas = int32(as.MaxReplicas)
	}
	cpuTarget := int32(80)
	if as.CPUTarget > 0 {
		cpuTarget = int32(as.CPUTarget)
	}

	hpa := autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       app.Name,
			},
			MinReplicas: &minReplicas,
			MaxReplicas: maxReplicas,
			Metrics: []autoscalingv2.MetricSpec{
				{
					Type: autoscalingv2.ResourceMetricSourceType,
					Resource: &autoscalingv2.ResourceMetricSource{
						Name: corev1.ResourceCPU,
						Target: autoscalingv2.MetricTarget{
							Type:               autoscalingv2.UtilizationMetricType,
							AverageUtilization: &cpuTarget,
						},
					},
				},
			},
		},
	}

	var existing autoscalingv2.HorizontalPodAutoscaler
	err := c.Get(ctx, client.ObjectKeyFromObject(&hpa), &existing)
	if errors.IsNotFound(err) {
		log.Info("creating HPA", "min", minReplicas, "max", maxReplicas, "cpuTarget", cpuTarget)
		return c.Create(ctx, &hpa)
	}
	if err != nil {
		return err
	}

	log.Info("updating HPA", "min", minReplicas, "max", maxReplicas, "cpuTarget", cpuTarget)
	hpa.ResourceVersion = existing.ResourceVersion
	return c.Update(ctx, &hpa)
}
