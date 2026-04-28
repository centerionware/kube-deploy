package controllers

import (
	"context"

	v1 "npm-operator/api/v1alpha1"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type NpmAppReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func mergeMap(user, defaults map[string]string) map[string]string {
	if user == nil {
		return defaults
	}

	out := map[string]string{}

	for k, v := range defaults {
		out[k] = v
	}

	for k, v := range user {
		out[k] = v
	}

	return out
}

func mergeString(userVal, defaultVal string) string {
	if userVal != "" {
		return userVal
	}
	return defaultVal
}

func (r *NpmAppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	l := log.FromContext(ctx)

	var app v1.NpmApp
	if err := r.Get(ctx, req.NamespacedName, &app); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if err := ensureKpackImage(ctx, r.Client, app); err != nil {
		return ctrl.Result{}, err
	}

	image, err := getLatestImageDigest(ctx, r.Client, app)
	if err != nil {
		return ctrl.Result{}, err
	}

	if image == "" {
		l.Info("waiting for kpack build")
		return ctrl.Result{}, nil
	}

	if err := ensureDeployment(ctx, r.Client, app, image); err != nil {
		return ctrl.Result{}, err
	}

	if err := ensureService(ctx, r.Client, app); err != nil {
		return ctrl.Result{}, err
	}

	app.Status.Image = image
	app.Status.Phase = "Ready"
	app.Status.ObservedGeneration = app.Generation

	_ = r.Status().Update(ctx, &app)

	return ctrl.Result{}, nil
}