package controllers

import (
	"context"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func upsertDeployment(ctx context.Context, c client.Client, obj appsv1.Deployment) error {
	log := log.FromContext(ctx).WithValues("deployment", obj.Name, "namespace", obj.Namespace)

	var existing appsv1.Deployment
	err := c.Get(ctx, client.ObjectKeyFromObject(&obj), &existing)
	if errors.IsNotFound(err) {
		log.Info("creating deployment")
		return c.Create(ctx, &obj)
	}
	if err != nil {
		log.Error(err, "failed to get deployment")
		return err
	}

	// Only update if something meaningful actually changed
	if reflect.DeepEqual(existing.Spec, obj.Spec) &&
		reflect.DeepEqual(existing.Labels, obj.Labels) &&
		reflect.DeepEqual(existing.Annotations, obj.Annotations) {
		log.Info("deployment unchanged, skipping update")
		return nil
	}

	log.Info("deployment changed, updating")
	obj.ResourceVersion = existing.ResourceVersion
	return c.Update(ctx, &obj)
}

func upsertService(ctx context.Context, c client.Client, obj corev1.Service) error {
	log := log.FromContext(ctx).WithValues("service", obj.Name, "namespace", obj.Namespace)

	var existing corev1.Service
	err := c.Get(ctx, client.ObjectKeyFromObject(&obj), &existing)
	if errors.IsNotFound(err) {
		log.Info("creating service")
		return c.Create(ctx, &obj)
	}
	if err != nil {
		log.Error(err, "failed to get service")
		return err
	}

	// Only update if something meaningful actually changed
	if reflect.DeepEqual(existing.Spec.Ports, obj.Spec.Ports) &&
		reflect.DeepEqual(existing.Spec.Selector, obj.Spec.Selector) &&
		existing.Spec.Type == obj.Spec.Type &&
		reflect.DeepEqual(existing.Annotations, obj.Annotations) &&
		reflect.DeepEqual(existing.Labels, obj.Labels) {
		log.Info("service unchanged, skipping update")
		return nil
	}

	log.Info("service changed, updating")
	obj.ResourceVersion = existing.ResourceVersion
	// ClusterIP is immutable — must preserve it
	obj.Spec.ClusterIP = existing.Spec.ClusterIP
	obj.Spec.ClusterIPs = existing.Spec.ClusterIPs
	return c.Update(ctx, &obj)
}
