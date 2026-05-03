package controllers

import (
	"context"
	"fmt"

	v1 "kube-deploy/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// EnsureVolumes creates any PVCs that need to exist.
// ConfigMap, Secret, EmptyDir, and HostPath volumes need no pre-creation.
func EnsureVolumes(ctx context.Context, c client.Client, app *v1.App) error {
	log := log.FromContext(ctx).WithValues("app", app.Name, "namespace", app.Namespace)

	for _, vol := range app.Spec.Run.Volumes {
		if vol.PVC == nil {
			continue // only PVCs need pre-creation
		}

		claimName := vol.PVC.ClaimName
		if claimName == "" {
			claimName = vol.Name // auto-name from volume name
		}

		size := vol.PVC.Size
		if size == "" {
			size = "1Gi"
		}
		storageClass := vol.PVC.StorageClass
		if storageClass == "" {
			storageClass = "local-path"
		}

		pvc := corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      claimName,
				Namespace: app.Namespace,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
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
			log.Info("creating PVC", "name", claimName, "size", size)
			if err := c.Create(ctx, &pvc); err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else {
			log.Info("PVC already exists", "name", claimName)
		}
	}

	return nil
}

// buildVolumes converts VolumeSpec slice into Kubernetes Volume and VolumeMount slices
func buildVolumes(specs []v1.VolumeSpec) ([]corev1.Volume, []corev1.VolumeMount, error) {
	var volumes []corev1.Volume
	var mounts []corev1.VolumeMount

	for _, spec := range specs {
		vol, err := buildVolume(spec)
		if err != nil {
			return nil, nil, err
		}
		volumes = append(volumes, vol)
		mounts = append(mounts, corev1.VolumeMount{
			Name:      spec.Name,
			MountPath: spec.MountPath,
			ReadOnly:  spec.PVC != nil && spec.PVC.ReadOnly,
		})
	}

	return volumes, mounts, nil
}

func buildVolume(spec v1.VolumeSpec) (corev1.Volume, error) {
	vol := corev1.Volume{Name: spec.Name}

	switch {
	case spec.PVC != nil:
		claimName := spec.PVC.ClaimName
		if claimName == "" {
			claimName = spec.Name
		}
		vol.VolumeSource = corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: claimName,
				ReadOnly:  spec.PVC.ReadOnly,
			},
		}

	case spec.ConfigMap != nil:
		cm := &corev1.ConfigMapVolumeSource{
			LocalObjectReference: corev1.LocalObjectReference{Name: spec.ConfigMap.Name},
		}
		for _, item := range spec.ConfigMap.Items {
			cm.Items = append(cm.Items, corev1.KeyToPath{
				Key:  item.Key,
				Path: item.Path,
			})
		}
		vol.VolumeSource = corev1.VolumeSource{ConfigMap: cm}

	case spec.Secret != nil:
		sv := &corev1.SecretVolumeSource{
			SecretName: spec.Secret.SecretName,
		}
		for _, item := range spec.Secret.Items {
			sv.Items = append(sv.Items, corev1.KeyToPath{
				Key:  item.Key,
				Path: item.Path,
			})
		}
		vol.VolumeSource = corev1.VolumeSource{Secret: sv}

	case spec.EmptyDir != nil:
		ed := &corev1.EmptyDirVolumeSource{}
		if spec.EmptyDir.Medium == "Memory" {
			ed.Medium = corev1.StorageMediumMemory
		}
		vol.VolumeSource = corev1.VolumeSource{EmptyDir: ed}

	case spec.HostPath != nil:
		hp := &corev1.HostPathVolumeSource{Path: spec.HostPath.Path}
		if spec.HostPath.Type != "" {
			t := corev1.HostPathType(spec.HostPath.Type)
			hp.Type = &t
		}
		vol.VolumeSource = corev1.VolumeSource{HostPath: hp}

	default:
		return vol, fmt.Errorf("volume %q has no source defined (set pvc, configMap, secret, emptyDir, or hostPath)", spec.Name)
	}

	return vol, nil
}
