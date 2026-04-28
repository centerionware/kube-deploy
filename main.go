package main

import (
	"npm-operator/api/v1alpha1"
	"npm-operator/controllers"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func main() {

	ctrl.SetLogger(zap.New())

	scheme := runtime.NewScheme()

	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(appsv1.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))

	// IMPORTANT: register CRD
	scheme.AddKnownTypes(v1alpha1.GroupVersion, &v1alpha1.NpmApp{})

	mgr, _ := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
	})

	reconciler := &controllers.NpmAppReconciler{
		Client: mgr.GetClient(),
		Scheme: scheme,
	}

	ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.NpmApp{}).
		Complete(reconciler)

	mgr.Start(ctrl.SetupSignalHandler())
}