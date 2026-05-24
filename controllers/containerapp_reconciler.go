package controllers

import (
	"context"
	"fmt"
	"runtime/debug"
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

func (r *ContainerAppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	log := log.FromContext(ctx).WithValues("containerapp", req.NamespacedName)

	// Recover from panics — bad CRs must never crash the worker goroutine
	defer func() {
		if rec := recover(); rec != nil {
			log.Error(fmt.Errorf("panic: %v\n%s", rec, debug.Stack()), "reconcile panicked, recovering")
			result = ctrl.Result{RequeueAfter: 60 * time.Second}
			err = nil
		}
	}()

	// Per-reconcile timeout
	ctx, cancel := context.WithTimeout(ctx, reconcileTimeout)
	defer cancel()

	log.Info("reconcile triggered")

	var app v1.ContainerApp
	if err := r.Get(ctx, req.NamespacedName, &app); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return ctrl.Result{}, nil
		}
		log.Error(err, "failed to get ContainerApp")
		return ctrl.Result{}, err
	}

	// --- Deletion ---
	if !app.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&app, containerAppFinalizer) {
			log.Info("ContainerApp deleted, cleaning up")
			if cleanupErr := r.cleanup(ctx, &app); cleanupErr != nil {
				log.Error(cleanupErr, "cleanup failed")
				return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
			}
			controllerutil.RemoveFinalizer(&app, containerAppFinalizer)
			if updateErr := r.Update(ctx, &app); updateErr != nil {
				return ctrl.Result{}, updateErr
			}
		}
		return ctrl.Result{}, nil
	}

	// --- Finalizer ---
	if !controllerutil.ContainsFinalizer(&app, containerAppFinalizer) {
		controllerutil.AddFinalizer(&app, containerAppFinalizer)
		if updateErr := r.Update(ctx, &app); updateErr != nil {
			return ctrl.Result{}, updateErr
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

	if runtimeErr := EnsureRuntime(ctx, r.Client, synthetic, app.Spec.Image); runtimeErr != nil {
		log.Error(runtimeErr, "EnsureRuntime failed", "image", app.Spec.Image)
		app.Status.Phase = "Failed"
		app.Status.Message = runtimeErr.Error()
		app.Status.LastUpdate = time.Now().Format(time.RFC3339)
		_ = r.Status().Update(ctx, &app)
		// Non-fatal — requeue with backoff
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	app.Status.Phase = "Ready"
	app.Status.Message = ""
	app.Status.LastUpdate = time.Now().Format(time.RFC3339)
	if statusErr := r.Status().Update(ctx, &app); statusErr != nil {
		log.Error(statusErr, "failed to update status")
	}

	log.Info("ContainerApp reconcile complete", "image", app.Spec.Image)
	return ctrl.Result{}, nil
}

func (r *ContainerAppReconciler) cleanup(ctx context.Context, app *v1.ContainerApp) error {
	log := log.FromContext(ctx).WithValues("containerapp", app.Name, "namespace", app.Namespace)

	synthetic := &v1.App{
		ObjectMeta: app.ObjectMeta,
		Spec: v1.AppSpec{
			Run:       app.Spec.Run,
			RBAC:      app.Spec.RBAC,
			Resources: app.Spec.Resources,
		},
	}

	if err := cleanupRuntime(ctx, r.Client, synthetic); err != nil {
		log.Error(err, "runtime cleanup failed (best-effort)")
	}

	if err := cleanupResources(ctx, r.Client, synthetic); err != nil {
		log.Error(err, "generic resources cleanup failed (best-effort)")
	}

	if err := cleanupRBAC(ctx, r.Client, synthetic); err != nil {
		log.Error(err, "RBAC cleanup failed (best-effort)")
	}

	log.Info("ContainerApp cleanup complete")
	return nil
}
