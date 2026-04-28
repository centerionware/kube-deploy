package controllers

import (
	"context"
	"time"

	v1 "npm-operator/api/v1alpha1"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type NpmAppReconciler struct {
	client.Client
}

func (r *NpmAppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	var app v1.NpmApp
	if err := r.Get(ctx, req.NamespacedName, &app); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	image, err := ensureBuildJob(ctx, r.Client, app)
	if err != nil {
		return ctrl.Result{}, err
	}

	// wait for build completion
	app.Status.Phase = "Building"
	app.Status.Image = image
	_ = r.Status().Update(ctx, &app)

	// naive wait loop (improve later)
	return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
}