package controllers

import (
	"context"
	"time"

	v1 "kube-deploy/api/v1alpha1"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const containerAppFinalizer = "kube-deploy.centerionware.app/container-finalizer"

type ContainerAppReconciler struct {
	client.Client
}

func SetupContainerApp(mgr ctrl.Manager, r *ContainerAppReconciler) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.ContainerApp{}).
		Complete(r)
}

func (r *ContainerAppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithValues("containerapp", req.NamespacedName)
	log.Info("reconcile triggered")

	var app v1.ContainerApp
	if err := r.Get(ctx, req.NamespacedName, &app); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return ctrl.Result{}, nil
		}
		log.Error(err, "failed to get ContainerApp")
		return ctrl.Result{}, err
	}

	if !app.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&app, containerAppFinalizer) {
			log.Info("ContainerApp deleted, cleaning up")
			if err := r.cleanup(ctx, &app); err != nil {
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(&app, containerAppFinalizer)
			if err := r.Update(ctx, &app); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(&app, containerAppFinalizer) {
		controllerutil.AddFinalizer(&app, containerAppFinalizer)
		if err := r.Update(ctx, &app); err != nil {
			return ctrl.Result{}, err
		}
	}

	log.Info("deploying container", "image", app.Spec.Image)

	synthetic := &v1.App{
		ObjectMeta: app.ObjectMeta,
		Spec: v1.AppSpec{
			Env:       app.Spec.Env,
			Run:       app.Spec.Run,
			Service:   app.Spec.Service,
			Ingress:   app.Spec.Ingress,
			Gateway:   app.Spec.Gateway,
			RBAC:      app.Spec.RBAC,
			Resources: app.Spec.Resources,
		},
	}

	if err := EnsureRuntime(ctx, r.Client, synthetic, app.Spec.Image); err != nil {
		log.Error(err, "EnsureRuntime failed", "image", app.Spec.Image)
		app.Status.Phase = "Failed"
		app.Status.Message = err.Error()
		app.Status.LastUpdate = time.Now().Format(time.RFC3339)
		_ = r.Status().Update(ctx, &app)
		return ctrl.Result{}, err
	}

	app.Status.Phase = "Ready"
	app.Status.Message = ""
	app.Status.LastUpdate = time.Now().Format(time.RFC3339)
	if err := r.Status().Update(ctx, &app); err != nil {
		log.Error(err, "failed to update status")
	}

	log.Info("ContainerApp reconcile complete", "image", app.Spec.Image)
	return ctrl.Result{}, nil
}

func (r *ContainerAppReconciler) cleanup(ctx context.Context, app *v1.ContainerApp) error {
	log := log.FromContext(ctx).WithValues("containerapp", app.Name, "namespace", app.Namespace)
	synthetic := &v1.App{ObjectMeta: app.ObjectMeta}
	if err := cleanupRuntime(ctx, r.Client, synthetic); err != nil {
		log.Error(err, "runtime cleanup failed")
		return err
	}
	log.Info("ContainerApp cleanup complete")
	return nil
}
