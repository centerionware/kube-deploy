package controllers

import (
	v1 "npm-operator/api/v1alpha1"

	ctrl "sigs.k8s.io/controller-runtime"
)

func SetupWithManager(mgr ctrl.Manager, r *NpmAppReconciler) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.NpmApp{}).
		Complete(r)
}