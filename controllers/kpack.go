package controllers

import (
	"context"

	v1 "npm-operator/api/v1alpha1"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ensureKpackImage(ctx context.Context, c client.Client, app v1.NpmApp) string {

	image := &unstructured.Unstructured{}
	image.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "kpack.io",
		Version: "v1alpha2",
		Kind:    "Image",
	})

	image.SetName(app.Name)
	image.SetNamespace(app.Namespace)

	image.Object["spec"] = map[string]interface{}{
		"tag": app.Spec.Image,
		"source": map[string]interface{}{
			"git": map[string]interface{}{
				"url":      app.Spec.Repo,
				"revision": app.Spec.Revision,
			},
		},
	}

	_ = c.Patch(ctx, image, client.Apply, client.ForceOwnership, client.FieldOwner("npm-operator"))

	return app.Spec.Image
}