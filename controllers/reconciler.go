package controllers

import (
	"context"
	"time"

	v1 "kube-deploy/api/v1alpha1"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type NpmAppReconciler struct {
	client.Client
}

func Setup(mgr ctrl.Manager, r *NpmAppReconciler) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.NpmApp{}).
		Complete(r)
}

func (r *NpmAppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithValues("npmapp", req.NamespacedName)

	log.Info("reconcile triggered")

	var app v1.NpmApp
	if err := r.Get(ctx, req.NamespacedName, &app); err != nil {
		if client.IgnoreNotFound(err) == nil {
			log.Info("resource not found, likely deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "failed to get NpmApp")
		return ctrl.Result{}, err
	}

	log.Info("fetched NpmApp", "repo", app.Spec.Repo, "phase", app.Status.Phase)

	image, ready, err := EnsureBuild(ctx, r.Client, &app)
	if err != nil {
		log.Error(err, "EnsureBuild failed", "repo", app.Spec.Repo)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	if !ready {
		log.Info("build not ready yet, requeuing", "repo", app.Spec.Repo)
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	log.Info("build ready", "image", image)

	if err := EnsureRuntime(ctx, r.Client, &app, image); err != nil {
		log.Error(err, "EnsureRuntime failed", "image", image)
		return ctrl.Result{}, err
	}

	log.Info("reconcile complete", "image", image, "requeueIn", "5m")
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}
