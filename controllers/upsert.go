package controllers

import (
	"context"

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
	log.Info("updating deployment")
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
	log.Info("updating service")
	obj.ResourceVersion = existing.ResourceVersion
	obj.Spec.ClusterIP = existing.Spec.ClusterIP
	obj.Spec.ClusterIPs = existing.Spec.ClusterIPs
	return c.Update(ctx, &obj)
}
