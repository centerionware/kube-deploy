package controllers

import (
	"context"
	"fmt"
	"time"

	v1 "npm-operator/api/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type NpmAppReconciler struct {
	client.Client
}

// ---------------- MAIN LOOP ----------------

func (r *NpmAppReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {

	var app v1.NpmApp
	if err := r.Get(ctx, req.NamespacedName, &app); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	image := resolveImage(app)

	if err := r.ensureDeployment(ctx, app, image); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.ensureService(ctx, app); err != nil {
		return reconcile.Result{}, err
	}

	app.Status.Phase = "Ready"
	app.Status.Image = image
	app.Status.JobName = fmt.Sprintf("%s-build", app.Name)

	_ = r.Status().Update(ctx, &app)

	return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
}

// ---------------- IMAGE ----------------

func resolveImage(app v1.NpmApp) string {
	if app.Spec.Build.OutputImage != "" {
		return app.Spec.Build.OutputImage
	}
	return fmt.Sprintf("registry.local/%s:latest", app.Name)
}

// ---------------- DEPLOYMENT ----------------

func (r *NpmAppReconciler) ensureDeployment(ctx context.Context, app v1.NpmApp, image string) error {

	deploy := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": app.Name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": app.Name},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "app",
							Image: image,
							Env:   buildEnv(app.Spec.Env),
							Ports: []corev1.ContainerPort{
								{ContainerPort: int32(app.Spec.Run.Port)},
							},
							Command: app.Spec.Run.Command,
						},
					},
				},
			},
		},
	}

	var existing appsv1.Deployment
	err := r.Get(ctx, client.ObjectKey{Name: app.Name, Namespace: app.Namespace}, &existing)

	if errors.IsNotFound(err) {
		return r.Create(ctx, &deploy)
	}
	if err != nil {
		return err
	}

	existing.Spec = deploy.Spec
	return r.Update(ctx, &existing)
}

// ---------------- SERVICE ----------------

func (r *NpmAppReconciler) ensureService(ctx context.Context, app v1.NpmApp) error {

	svc := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        app.Name,
			Namespace:   app.Namespace,
			Annotations: app.Spec.Service.Annotations,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": app.Name},
			Ports: []corev1.ServicePort{
				{
					Port: int32(app.Spec.Run.Port),
				},
			},
		},
	}

	var existing corev1.Service
	err := r.Get(ctx, client.ObjectKey{Name: app.Name, Namespace: app.Namespace}, &existing)

	if errors.IsNotFound(err) {
		return r.Create(ctx, &svc)
	}
	if err != nil {
		return err
	}

	existing.Spec = svc.Spec
	existing.Annotations = svc.Annotations

	return r.Update(ctx, &existing)
}

// ---------------- HELPERS ----------------

func int32Ptr(i int32) *int32 { return &i }

func buildEnv(env map[string]string) []corev1.EnvVar {
	var out []corev1.EnvVar
	for k, v := range env {
		out = append(out, corev1.EnvVar{Name: k, Value: v})
	}
	return out
}