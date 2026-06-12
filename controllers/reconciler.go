package controllers

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	v1 "kube-deploy/api/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	appFinalizer        = "kube-deploy.centerionware.app/finalizer"
	defaultPollInterval = 1 * time.Minute
	reconcileTimeout    = 5 * time.Minute
)

type AppReconciler struct {
	client.Client
}

func Setup(mgr ctrl.Manager, r *AppReconciler) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.App{}).
		Complete(r)
}

func (r *AppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	log := log.FromContext(ctx).WithValues("app", req.NamespacedName)

	// Recover from panics so a bad CR never crashes the worker goroutine
	defer func() {
		if rec := recover(); rec != nil {
			log.Error(fmt.Errorf("panic: %v\n%s", rec, debug.Stack()), "reconcile panicked, recovering")
			result = ctrl.Result{RequeueAfter: 60 * time.Second}
			err = nil
		}
	}()

	// Per-reconcile timeout so a stuck item never blocks the queue indefinitely
	ctx, cancel := context.WithTimeout(ctx, reconcileTimeout)
	defer cancel()

	log.Info("reconcile triggered")

	var app v1.App
	if err := r.Get(ctx, req.NamespacedName, &app); err != nil {
		if client.IgnoreNotFound(err) == nil {
			log.Info("resource not found, likely deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "failed to get App")
		return ctrl.Result{}, err
	}

	pollInterval := parsePollInterval(app.Spec.UpdateInterval)

	// --- Deletion ---
	if !app.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&app, appFinalizer) {
			log.Info("App deleted, running cleanup")
			if cleanupErr := r.cleanup(ctx, &app); cleanupErr != nil {
				log.Error(cleanupErr, "cleanup failed")
				return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
			}
			controllerutil.RemoveFinalizer(&app, appFinalizer)
			if updateErr := r.Update(ctx, &app); updateErr != nil {
				return ctrl.Result{}, updateErr
			}
		}
		return ctrl.Result{}, nil
	}

	// --- Finalizer ---
	if !controllerutil.ContainsFinalizer(&app, appFinalizer) {
		log.Info("adding finalizer")
		controllerutil.AddFinalizer(&app, appFinalizer)
		if updateErr := r.Update(ctx, &app); updateErr != nil {
			return ctrl.Result{}, updateErr
		}
	}

	log.Info("fetched App", "repo", app.Spec.Repo, "phase", app.Status.Phase)

	// --- Build ---
	image, ready, buildErr := EnsureBuild(ctx, r.Client, &app)
	if buildErr != nil {
		log.Error(buildErr, "EnsureBuild failed", "repo", app.Spec.Repo)
		// Non-fatal — requeue with backoff, don't block other CRs
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
	if !ready {
		if app.Status.PendingCommit != "" {
			log.Info("build not ready, pending commit queued, requeuing fast")
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		log.Info("build not ready, requeuing", "repo", app.Spec.Repo)
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}
	log.Info("build ready", "image", image)

	// --- Runtime ---
	if runtimeErr := EnsureRuntime(ctx, r.Client, &app, image); runtimeErr != nil {
		log.Error(runtimeErr, "EnsureRuntime failed", "image", image)
		// Non-fatal — requeue with backoff
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// --- Job GC ---
	if gcErr := r.cleanupOldJobs(ctx, &app); gcErr != nil {
		log.Error(gcErr, "job cleanup failed (non-fatal)")
	}

	log.Info("reconcile complete", "image", image, "nextPoll", pollInterval)
	return ctrl.Result{RequeueAfter: pollInterval}, nil
}

func parsePollInterval(s string) time.Duration {
	if s == "" {
		return defaultPollInterval
	}
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return defaultPollInterval
	}
	return d
}

func (r *AppReconciler) cleanup(ctx context.Context, app *v1.App) error {
	log := log.FromContext(ctx).WithValues("app", app.Name, "namespace", app.Namespace)

	var deploy appsv1.Deployment
	if err := r.Get(ctx, client.ObjectKey{Name: app.Name, Namespace: app.Namespace}, &deploy); err == nil {
		log.Info("deleting deployment")
		if err := r.Delete(ctx, &deploy); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("deleting deployment: %w", err)
		}
	}

	var svc corev1.Service
	if err := r.Get(ctx, client.ObjectKey{Name: app.Name, Namespace: app.Namespace}, &svc); err == nil {
		log.Info("deleting service")
		if err := r.Delete(ctx, &svc); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("deleting service: %w", err)
		}
	}

	var ing networkingv1.Ingress
	if err := r.Get(ctx, client.ObjectKey{Name: app.Name, Namespace: app.Namespace}, &ing); err == nil {
		log.Info("deleting ingress")
		if err := r.Delete(ctx, &ing); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("deleting ingress: %w", err)
		}
	}

	if err := deleteHTTPRoute(ctx, r.Client, app.Name, app.Namespace); err != nil {
		log.Error(err, "failed to delete HTTPRoute (best-effort)")
	}

	var hpa autoscalingv2.HorizontalPodAutoscaler
	if err := r.Get(ctx, client.ObjectKey{Name: app.Name, Namespace: app.Namespace}, &hpa); err == nil {
		log.Info("deleting HPA")
		if err := r.Delete(ctx, &hpa); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("deleting HPA: %w", err)
		}
	}

	var jobList batchv1.JobList
	if err := r.List(ctx, &jobList,
		client.InNamespace(app.Namespace),
		client.MatchingLabels{"kube-deploy/app": app.Name},
	); err == nil {
		propagation := client.PropagationPolicy("Background")
		for _, job := range jobList.Items {
			log.Info("deleting build job", "job", job.Name)
			if err := r.Delete(ctx, &job, propagation); err != nil && !errors.IsNotFound(err) {
				log.Error(err, "failed to delete job", "job", job.Name)
			}
		}
	}

	if err := cleanupResources(ctx, r.Client, app); err != nil {
		log.Error(err, "generic resources cleanup failed (best-effort)")
	}

	if err := cleanupRBAC(ctx, r.Client, app); err != nil {
		log.Error(err, "RBAC cleanup failed (best-effort)")
	}

	if err := deleteRegistryImage(ctx, app); err != nil {
		log.Error(err, "registry cleanup failed (best-effort)")
	}

	log.Info("cleanup complete")
	return nil
}

func (r *AppReconciler) cleanupOldJobs(ctx context.Context, app *v1.App) error {
	log := log.FromContext(ctx).WithValues("app", app.Name, "namespace", app.Namespace)

	var jobList batchv1.JobList
	if err := r.List(ctx, &jobList,
		client.InNamespace(app.Namespace),
		client.MatchingLabels{"kube-deploy/app": app.Name},
	); err != nil {
		return err
	}

	currentJobName := jobNameFromImage(app.Name, app.Status.Image)

	propagation := client.PropagationPolicy("Background")
	for _, job := range jobList.Items {
		if job.Status.Succeeded > 0 || job.Status.Failed > 0 {
			if job.Name == currentJobName {
				continue
			}
			log.Info("cleaning up old job", "job", job.Name)
			if err := r.Delete(ctx, &job, propagation); err != nil && !errors.IsNotFound(err) {
				log.Error(err, "failed to delete old job", "job", job.Name)
			}
		}
	}
	return nil
}
