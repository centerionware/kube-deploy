package controllers

import (
	v1 "npm-operator/api/v1alpha1"
)

func setRunning(app *v1.NpmApp, image string, job string) {
	app.Status.Phase = "Running"
	app.Status.Image = image
	app.Status.JobName = job
}