package controllers

import (
	"context"

	v1 "npm-operator/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func EnsureVolumes(ctx context.Context, c client.Client, app *v1.NpmApp) error {
	log := log.FromContext(ctx).WithValues("npmapp", app.Name, "namespace", app.Namespace)

	for _, vol := range app.Spec.Run.Volumes {
		size := vol.Size
		if size == "" {
			size = "1Gi"
		}
		storageClass := vol.StorageClass
		if storageClass == "" {
			storageClass = "local-path"
		}

		pvc := corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      vol.Name,
				Namespace: app.Namespace,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
				StorageClassName: &storageClass,
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse(size),
					},
				},
			},
		}

		var existing corev1.PersistentVolumeClaim
		err := c.Get(ctx, client.ObjectKeyFromObject(&pvc), &existing)
		if errors.IsNotFound(err) {
			log.Info("creating PVC", "name", vol.Name, "size", size, "storageClass", storageClass)
			if err := c.Create(ctx, &pvc); err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else {
			log.Info("PVC already exists", "name", vol.Name)
		}
		// PVCs are not updated after creation — immutable spec
	}

	return nil
}
