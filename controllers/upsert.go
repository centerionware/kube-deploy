package controllers

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func upsertDeployment(ctx context.Context, c client.Client, obj appsv1.Deployment) error {

	var existing appsv1.Deployment

	err := c.Get(ctx, client.ObjectKeyFromObject(&obj), &existing)
	if errors.IsNotFound(err) {
		return c.Create(ctx, &obj)
	}
	if err != nil {
		return err
	}

	obj.ResourceVersion = existing.ResourceVersion
	return c.Update(ctx, &obj)
}

func upsertService(ctx context.Context, c client.Client, obj corev1.Service) error {

	var existing corev1.Service

	err := c.Get(ctx, client.ObjectKeyFromObject(&obj), &existing)
	if errors.IsNotFound(err) {
		return c.Create(ctx, &obj)
	}
	if err != nil {
		return err
	}

	obj.ResourceVersion = existing.ResourceVersion
	return c.Update(ctx, &obj)
}