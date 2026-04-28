package controllers

import (
	"context"

	v1 "npm-operator/api/v1alpha1"

	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type NpmAppReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *NpmAppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	l := log.FromContext(ctx)

	var app v1.NpmApp
	if err := r.Get(ctx, req.NamespacedName, &app); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// ----------------------------------------
	// FINALIZER (safe cleanup hook)
	// ----------------------------------------
	if app.ObjectMeta.DeletionTimestamp.IsZero() {
		if !containsFinalizer(app.Finalizers, "npm.finalizers.centerionware.app") {
			app.Finalizers = append(app.Finalizers, "npm.finalizers.centerionware.app")
			_ = r.Update(ctx, &app)
			return ctrl.Result{}, nil
		}
	} else {
		// cleanup logic could go here
		return ctrl.Result{}, nil
	}

	// ----------------------------------------
	// 1. Ensure kpack Image
	// ----------------------------------------
	imageName := ensureKpackImage(ctx, r.Client, app)

	// ----------------------------------------
	// 2. Deployment
	// ----------------------------------------
	err := ensureDeployment(ctx, r.Client, app, imageName)
	if err != nil {
		l.Error(err, "deployment failed")
		return ctrl.Result{}, err
	}

	// ----------------------------------------
	// 3. Service
	// ----------------------------------------
	err = ensureService(ctx, r.Client, app)
	if err != nil {
		return ctrl.Result{}, err
	}

	// ----------------------------------------
	// STATUS UPDATE
	// ----------------------------------------
	app.Status.Image = imageName
	app.Status.Phase = "Ready"
	app.Status.ObservedGeneration = app.Generation

	_ = r.Status().Update(ctx, &app)

	return ctrl.Result{}, nil
}

func containsFinalizer(list []string, f string) bool {
	for _, v := range list {
		if v == f {
			return true
		}
	}
	return false
}