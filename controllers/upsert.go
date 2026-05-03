package controllers

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func upsertDeployment(ctx context.Context, c client.Client, desired appsv1.Deployment) error {
	log := log.FromContext(ctx).WithValues("deployment", desired.Name, "namespace", desired.Namespace)

	var existing appsv1.Deployment
	err := c.Get(ctx, client.ObjectKeyFromObject(&desired), &existing)
	if errors.IsNotFound(err) {
		log.Info("creating deployment")
		return c.Create(ctx, &desired)
	}
	if err != nil {
		return err
	}

	// Compare only the fields we own — ignore k8s-injected metadata
	existingImage := ""
	existingReplicas := int32(0)
	if len(existing.Spec.Template.Spec.Containers) > 0 {
		existingImage = existing.Spec.Template.Spec.Containers[0].Image
	}
	if existing.Spec.Replicas != nil {
		existingReplicas = *existing.Spec.Replicas
	}

	desiredImage := ""
	desiredReplicas := int32(0)
	if len(desired.Spec.Template.Spec.Containers) > 0 {
		desiredImage = desired.Spec.Template.Spec.Containers[0].Image
	}
	if desired.Spec.Replicas != nil {
		desiredReplicas = *desired.Spec.Replicas
	}

	podSpecChanged := existingImage != desiredImage ||
		existingReplicas != desiredReplicas ||
		podTemplateChanged(existing.Spec.Template, desired.Spec.Template)

	if !podSpecChanged {
		log.Info("deployment unchanged, skipping update")
		return nil
	}

	log.Info("deployment changed, updating")
	existing.Spec.Replicas = desired.Spec.Replicas
	existing.Spec.Template = desired.Spec.Template
	existing.Labels = desired.Labels
	return c.Update(ctx, &existing)
}

// podTemplateChanged compares the fields we control in the pod template
func podTemplateChanged(existing, desired corev1.PodTemplateSpec) bool {
	if len(existing.Spec.Containers) != len(desired.Spec.Containers) {
		return true
	}
	for i := range desired.Spec.Containers {
		e := existing.Spec.Containers[i]
		d := desired.Spec.Containers[i]
		if e.Image != d.Image {
			return true
		}
		if e.Name != d.Name {
			return true
		}
		if !envEqual(e.Env, d.Env) {
			return true
		}
		if !resourcesEqual(e.Resources, d.Resources) {
			return true
		}
		if !portsEqual(e.Ports, d.Ports) {
			return true
		}
		if !commandEqual(e.Command, d.Command) {
			return true
		}
		if !commandEqual(e.Args, d.Args) {
			return true
		}
	}
	if !volumesEqual(existing.Spec.Volumes, desired.Spec.Volumes) {
		return true
	}
	return false
}

func envEqual(a, b []corev1.EnvVar) bool {
	if len(a) != len(b) {
		return false
	}
	m := make(map[string]string, len(a))
	for _, e := range a {
		m[e.Name] = e.Value
	}
	for _, e := range b {
		if m[e.Name] != e.Value {
			return false
		}
	}
	return true
}

func resourcesEqual(a, b corev1.ResourceRequirements) bool {
	for _, r := range []corev1.ResourceName{corev1.ResourceCPU, corev1.ResourceMemory} {
		if a.Requests[r] != b.Requests[r] {
			return false
		}
		if a.Limits[r] != b.Limits[r] {
			return false
		}
	}
	return true
}

func portsEqual(a, b []corev1.ContainerPort) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ContainerPort != b[i].ContainerPort {
			return false
		}
	}
	return true
}

func commandEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func volumesEqual(a, b []corev1.Volume) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name {
			return false
		}
	}
	return true
}

func upsertService(ctx context.Context, c client.Client, desired corev1.Service) error {
	log := log.FromContext(ctx).WithValues("service", desired.Name, "namespace", desired.Namespace)

	var existing corev1.Service
	err := c.Get(ctx, client.ObjectKeyFromObject(&desired), &existing)
	if errors.IsNotFound(err) {
		log.Info("creating service")
		return c.Create(ctx, &desired)
	}
	if err != nil {
		return err
	}

	// Compare only the fields we control
	if portsSpecEqual(existing.Spec.Ports, desired.Spec.Ports) &&
		existing.Spec.Type == desired.Spec.Type &&
		annotationsEqual(existing.Annotations, desired.Annotations) {
		log.Info("service unchanged, skipping update")
		return nil
	}

	log.Info("service changed, updating")
	// Preserve immutable fields
	desired.Spec.ClusterIP = existing.Spec.ClusterIP
	desired.Spec.ClusterIPs = existing.Spec.ClusterIPs
	desired.ResourceVersion = existing.ResourceVersion
	return c.Update(ctx, &desired)
}

func portsSpecEqual(a, b []corev1.ServicePort) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Port != b[i].Port ||
			a[i].Protocol != b[i].Protocol ||
			a[i].TargetPort != b[i].TargetPort {
			return false
		}
	}
	return true
}

func annotationsEqual(a, b map[string]string) bool {
	for k, v := range b {
		if a[k] != v {
			return false
		}
	}
	return true
}
