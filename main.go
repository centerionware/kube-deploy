package main

import (
	"os"

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

	if err := mgr.Start(manager.SetupSignalHandler()); err != nil {
		os.Exit(1)
	}
}