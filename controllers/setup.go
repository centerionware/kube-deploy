package controllers

import (
	ctrl "sigs.k8s.io/controller-runtime"
)

func SetupWithManager(mgr ctrl.Manager, r *NpmAppReconciler) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&NpmApp{}).
		Complete(r)
}