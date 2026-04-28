package controllers

import (
	"context"
	"fmt"
	"time"

	v1 "npm-operator/api/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ---------------- RECONCILER ----------------

type NpmAppReconciler struct {
	client.Client
}

// ---------------- MAIN LOOP ----------------

func (r *NpmAppReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {

	var app v1.NpmApp
	if err := r.Get(ctx, req.NamespacedName, &app); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	commit, err := getGitSHA(app)
	if err != nil {
		app.Status.Phase = "Failed"
		_ = r.Status().Update(ctx, &app)
		return reconcile.Result{}, err
	}

	image := resolveImage(app)

	done, err := ensureBuildJob(ctx, r.Client, app, image, commit)
	if err != nil {
		return reconcile.Result{}, err
	}

	if !done {
		app.Status.Phase = "Building"
		app.Status.Commit = commit
		_ = r.Status().Update(ctx, &app)

		return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
	}

	if err := r.ensureDeployment(ctx, app, image); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.ensureService(ctx, app); err != nil {
		return reconcile.Result{}, err
	}

	app.Status.Phase = "Ready"
	app.Status.Image = image
	app.Status.Commit = commit

	_ = r.Status().Update(ctx, &app)

	return reconcile.Result{}, nil
}

// ---------------- IMAGE ----------------

func resolveImage(app v1.NpmApp) string {
	return fmt.Sprintf(
		"registry.registry.svc.cluster.local:5000/apps/%s:latest",
		app.Name,
	)
}

// ---------------- DEPLOYMENT ----------------

func (r *NpmAppReconciler) ensureDeployment(ctx context.Context, app v1.NpmApp, image string) error {

	deploy := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptrInt32(1),
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
							Ports: []corev1.ContainerPort{
								{ContainerPort: int32(app.Spec.Run.Port)},
							},
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
