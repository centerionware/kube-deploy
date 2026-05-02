package controllers

import (
	"context"
	"fmt"

	v1 "npm-operator/api/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// cleanupRuntime deletes Deployment, Service, Ingress, and HPA for an app.
// Used by both NpmApp and ContainerApp on deletion.
func cleanupRuntime(ctx context.Context, c client.Client, app *v1.NpmApp) error {
	log := log.FromContext(ctx).WithValues("app", app.Name, "namespace", app.Namespace)

	// Deployment
	var deploy appsv1.Deployment
	if err := c.Get(ctx, client.ObjectKey{Name: app.Name, Namespace: app.Namespace}, &deploy); err == nil {
		log.Info("deleting deployment")
		if err := c.Delete(ctx, &deploy); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("deleting deployment: %w", err)
		}
	}

	// Service
	var svc corev1.Service
	if err := c.Get(ctx, client.ObjectKey{Name: app.Name, Namespace: app.Namespace}, &svc); err == nil {
		log.Info("deleting service")
		if err := c.Delete(ctx, &svc); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("deleting service: %w", err)
		}
	}

	// Ingress
	var ing networkingv1.Ingress
	if err := c.Get(ctx, client.ObjectKey{Name: app.Name, Namespace: app.Namespace}, &ing); err == nil {
		log.Info("deleting ingress")
		if err := c.Delete(ctx, &ing); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("deleting ingress: %w", err)
		}
	}

	// HPA
	var hpa autoscalingv2.HorizontalPodAutoscaler
	if err := c.Get(ctx, client.ObjectKey{Name: app.Name, Namespace: app.Namespace}, &hpa); err == nil {
		log.Info("deleting HPA")
		if err := c.Delete(ctx, &hpa); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("deleting HPA: %w", err)
		}
	}

	return nil
}
