package controllers

import (
	"context"
	"encoding/json"
	"fmt"

	v1 "kube-deploy/api/v1alpha1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// EnsureResources applies each raw resource in spec.resources to the cluster.
// Resources are labeled with kube-deploy/app for ownership tracking and cleanup.
func EnsureResources(ctx context.Context, c client.Client, app *v1.App) error {
	log := log.FromContext(ctx).WithValues("app", app.Name, "namespace", app.Namespace)

	for i, raw := range app.Spec.Resources {
		obj := &unstructured.Unstructured{}
		if err := json.Unmarshal(raw, obj); err != nil {
			return fmt.Errorf("resource[%d]: invalid JSON/YAML: %w", i, err)
		}

		// Default namespace to app namespace if not set
		if obj.GetNamespace() == "" {
			obj.SetNamespace(app.Namespace)
		}

		// Inject ownership label so we can find and clean up later
		labels := obj.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		labels["kube-deploy/app"] = app.Name
		labels["kube-deploy/namespace"] = app.Namespace
		obj.SetLabels(labels)

		gvk := obj.GroupVersionKind()
		log.Info("applying resource",
			"apiVersion", gvk.GroupVersion().String(),
			"kind", gvk.Kind,
			"name", obj.GetName(),
			"namespace", obj.GetNamespace(),
		)

		// Server-side apply — works for any resource including CRDs
		data, err := json.Marshal(obj)
		if err != nil {
			return fmt.Errorf("resource[%d] %s/%s: marshal error: %w", i, gvk.Kind, obj.GetName(), err)
		}

		if err := c.Patch(ctx, obj, client.RawPatch(types.ApplyPatchType, data), 
			client.ForceOwnership,
			client.FieldOwner("kube-deploy"),
		); err != nil {
			return fmt.Errorf("resource[%d] %s/%s: apply failed: %w", i, gvk.Kind, obj.GetName(), err)
		}

		log.Info("resource applied",
			"kind", gvk.Kind,
			"name", obj.GetName(),
		)
	}

	return nil
}

// cleanupResources deletes all resources that were applied by this app
func cleanupResources(ctx context.Context, c client.Client, app *v1.App) error {
	log := log.FromContext(ctx).WithValues("app", app.Name, "namespace", app.Namespace)

	for i, raw := range app.Spec.Resources {
		obj := &unstructured.Unstructured{}
		if err := json.Unmarshal(raw, obj); err != nil {
			log.Error(err, "failed to parse resource for cleanup", "index", i)
			continue
		}

		if obj.GetNamespace() == "" {
			obj.SetNamespace(app.Namespace)
		}

		gvk := obj.GroupVersionKind()
		log.Info("deleting resource",
			"kind", gvk.Kind,
			"name", obj.GetName(),
			"namespace", obj.GetNamespace(),
		)

		// Need to set GVK for the dynamic client to find the right REST mapping
		obj.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   gvk.Group,
			Version: gvk.Version,
			Kind:    gvk.Kind,
		})

		if err := c.Delete(ctx, obj); err != nil && !errors.IsNotFound(err) {
			log.Error(err, "failed to delete resource", "kind", gvk.Kind, "name", obj.GetName())
		}
	}

	return nil
}
