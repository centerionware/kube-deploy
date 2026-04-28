package controllers

import (
	"context"
	"fmt"

	v1 "npm-operator/api/v1alpha1"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultGitRevision = "main"
	localRegistry      = "registry.registry.svc.cluster.local:5000"
)

func ensureKpackImage(ctx context.Context, c client.Client, app v1.NpmApp) error {

	imageName := fmt.Sprintf("%s/%s:latest", localRegistry, app.Name)
	gitRevision := defaultGitRevision

	img := &unstructured.Unstructured{}
	img.SetAPIVersion("kpack.io/v1alpha2")
	img.SetKind("Image")

	key := types.NamespacedName{
		Name:      app.Name,
		Namespace: app.Namespace,
	}

	err := c.Get(ctx, key, img)
	if err == nil {
		return nil
	}

	// create new kpack image
	img.Object = map[string]interface{}{
		"apiVersion": "kpack.io/v1alpha2",
		"kind":       "Image",
		"metadata": map[string]interface{}{
			"name":      app.Name,
			"namespace": app.Namespace,
		},
		"spec": map[string]interface{}{
			"tag": imageName,
			"builder": map[string]interface{}{
				"name": "default-builder",
				"kind": "ClusterBuilder",
			},
			"source": map[string]interface{}{
				"git": map[string]interface{}{
					"url":      app.Spec.Repo,
					"revision": gitRevision,
				},
			},
		},
	}

	return c.Create(ctx, img)
}

func getLatestImageDigest(ctx context.Context, c client.Client, app v1.NpmApp) (string, error) {

	img := &unstructured.Unstructured{}
	img.SetAPIVersion("kpack.io/v1alpha2")
	img.SetKind("Image")

	err := c.Get(ctx, types.NamespacedName{
		Name:      app.Name,
		Namespace: app.Namespace,
	}, img)

	if err != nil {
		return "", err
	}

	status, found, _ := unstructured.NestedMap(img.Object, "status", "latestImage")
	if !found {
		return "", nil
	}

	image, found, _ := unstructured.NestedString(status, "image")
	if !found {
		return "", nil
	}

	return image, nil
}