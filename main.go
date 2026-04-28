package main

import (
	"os"

	ctrl "sigs.k8s.io/controller-runtime"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	"npm-operator/api/v1alpha1"
	"npm-operator/controllers"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
}

func main() {

	ctrl.SetLogger(ctrl.Log.WithName("npm-operator"))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
	})
	if err != nil {
		os.Exit(1)
	}

	reconciler := &controllers.NpmAppReconciler{
		Client: mgr.GetClient(),
	}

	if err := controllers.SetupWithManager(mgr, reconciler); err != nil {
		os.Exit(1)
	}

	ctrl.Log.Info("starting manager")

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		os.Exit(1)
	}
}