package controllers

import (
	"context"

	v1 "kube-deploy/api/v1alpha1"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func EnsureHPA(ctx context.Context, c client.Client, app *v1.App) error {
	log := log.FromContext(ctx).WithValues("app", app.Name, "namespace", app.Namespace)

	if app.Spec.Run.Autoscaling == nil || !app.Spec.Run.Autoscaling.Enabled {
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

	desired := autoscalingv2.HorizontalPodAutoscaler{
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
	err := c.Get(ctx, client.ObjectKeyFromObject(&desired), &existing)
	if errors.IsNotFound(err) {
		log.Info("creating HPA", "min", minReplicas, "max", maxReplicas, "cpuTarget", cpuTarget)
		return c.Create(ctx, &desired)
	}
	if err != nil {
		return err
	}

	// Compare only fields we control
	if hpaSpecEqual(existing.Spec, desired.Spec) {
		log.Info("HPA unchanged, skipping update")
		return nil
	}

	log.Info("HPA changed, updating", "min", minReplicas, "max", maxReplicas, "cpuTarget", cpuTarget)
	desired.ResourceVersion = existing.ResourceVersion
	return c.Update(ctx, &desired)
}

func hpaSpecEqual(a, b autoscalingv2.HorizontalPodAutoscalerSpec) bool {
	if a.MaxReplicas != b.MaxReplicas {
		return false
	}
	if (a.MinReplicas == nil) != (b.MinReplicas == nil) {
		return false
	}
	if a.MinReplicas != nil && *a.MinReplicas != *b.MinReplicas {
		return false
	}
	if len(a.Metrics) != len(b.Metrics) {
		return false
	}
	for i := range a.Metrics {
		am := a.Metrics[i]
		bm := b.Metrics[i]
		if am.Type != bm.Type {
			return false
		}
		if am.Resource != nil && bm.Resource != nil {
			if am.Resource.Name != bm.Resource.Name {
				return false
			}
			if am.Resource.Target.AverageUtilization != nil &&
				bm.Resource.Target.AverageUtilization != nil &&
				*am.Resource.Target.AverageUtilization != *bm.Resource.Target.AverageUtilization {
				return false
			}
		}
	}
	return true
}