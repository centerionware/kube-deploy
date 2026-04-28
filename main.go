package main

import (
	"os"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"npm-operator/controllers"
)

func main() {

	cfg := config.GetConfigOrDie()

	mgr, err := manager.New(cfg, manager.Options{})
	if err != nil {
		os.Exit(1)
	}

	reconciler := &controllers.NpmAppReconciler{
		Client: mgr.GetClient(),
	}

	if err := controllers.SetupWithManager(mgr, reconciler); err != nil {
		os.Exit(1)
	}

	// FIX: correct signal handler for v0.17+
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		os.Exit(1)
	}
}