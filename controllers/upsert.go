package controllers

import (
	"context"
	"fmt"

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

	imageChanged := existingImage != desiredImage
	replicasChanged := existingReplicas != desiredReplicas
	templateChanged := podTemplateChanged(existing.Spec.Template, desired.Spec.Template)

	if !imageChanged && !replicasChanged && !templateChanged {
		log.Info("deployment unchanged, skipping update")
		return nil
	}

	log.Info("deployment changed, updating",
		"imageChanged", imageChanged,
		"replicasChanged", replicasChanged,
		"templateChanged", templateChanged,
	)
	existing.Spec.Replicas = desired.Spec.Replicas
	existing.Spec.Template = desired.Spec.Template
	existing.Labels = desired.Labels
	return c.Update(ctx, &existing)
}

// podTemplateChanged compares only the fields we set — ignores k8s-injected defaults
func podTemplateChanged(existing, desired corev1.PodTemplateSpec) bool {
	if len(existing.Spec.Containers) != len(desired.Spec.Containers) {
		fmt.Printf("[kube-deploy] templateChanged: container count %d vs %d\n",
			len(existing.Spec.Containers), len(desired.Spec.Containers))
		return true
	}
	for i := range desired.Spec.Containers {
		e := existing.Spec.Containers[i]
		d := desired.Spec.Containers[i]
		if e.Image != d.Image {
			fmt.Printf("[kube-deploy] templateChanged: image %q vs %q\n", e.Image, d.Image)
			return true
		}
		if e.Name != d.Name {
			fmt.Printf("[kube-deploy] templateChanged: name %q vs %q\n", e.Name, d.Name)
			return true
		}
		if !envEqual(e.Env, d.Env) {
			fmt.Printf("[kube-deploy] templateChanged: env\n")
			return true
		}
		if !commandEqual(e.Command, d.Command) {
			fmt.Printf("[kube-deploy] templateChanged: command %v vs %v\n", e.Command, d.Command)
			return true
		}
		if !commandEqual(e.Args, d.Args) {
			fmt.Printf("[kube-deploy] templateChanged: args %v vs %v\n", e.Args, d.Args)
			return true
		}
		if !portsEqual(e.Ports, d.Ports) {
			fmt.Printf("[kube-deploy] templateChanged: ports\n")
			return true
		}
		// Only compare the specific resource values we set, not k8s-defaulted ones
		if !setResourcesEqual(e.Resources, d.Resources) {
			fmt.Printf("[kube-deploy] templateChanged: resources\n")
			return true
		}
		if d.ReadinessProbe != nil {
			if e.ReadinessProbe == nil {
				fmt.Printf("[kube-deploy] templateChanged: readinessProbe nil\n")
				return true
			}
			if !probeEqual(e.ReadinessProbe, d.ReadinessProbe) {
				fmt.Printf("[kube-deploy] templateChanged: readinessProbe handler\n")
				return true
			}
		}
		if d.LivenessProbe != nil {
			if e.LivenessProbe == nil {
				fmt.Printf("[kube-deploy] templateChanged: livenessProbe nil\n")
				return true
			}
			if !probeEqual(e.LivenessProbe, d.LivenessProbe) {
				fmt.Printf("[kube-deploy] templateChanged: livenessProbe handler\n")
				return true
			}
		}
		if !desiredVolumeMountsPresent(e.VolumeMounts, d.VolumeMounts) {
			fmt.Printf("[kube-deploy] templateChanged: volumeMounts\n")
			return true
		}
	}
	if !desiredVolumesPresent(existing.Spec.Volumes, desired.Spec.Volumes) {
		fmt.Printf("[kube-deploy] templateChanged: volumes\n")
		return true
	}
	if desired.Spec.ServiceAccountName != "" &&
		existing.Spec.ServiceAccountName != desired.Spec.ServiceAccountName {
		fmt.Printf("[kube-deploy] templateChanged: serviceAccountName %q vs %q\n",
			existing.Spec.ServiceAccountName, desired.Spec.ServiceAccountName)
		return true
	}
	if existing.Spec.HostNetwork != desired.Spec.HostNetwork {
		fmt.Printf("[kube-deploy] templateChanged: hostNetwork %v vs %v\n",
			existing.Spec.HostNetwork, desired.Spec.HostNetwork)
		return true
	}
	return false
}

// probeEqual compares only the handler type and key fields, ignoring k8s-defaulted timing fields
func probeEqual(a, b *corev1.Probe) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	// Compare handler type
	if (a.HTTPGet == nil) != (b.HTTPGet == nil) {
		return false
	}
	if a.HTTPGet != nil && b.HTTPGet != nil {
		if a.HTTPGet.Path != b.HTTPGet.Path || a.HTTPGet.Port != b.HTTPGet.Port {
			return false
		}
	}
	if (a.TCPSocket == nil) != (b.TCPSocket == nil) {
		return false
	}
	if a.TCPSocket != nil && b.TCPSocket != nil {
		if a.TCPSocket.Port != b.TCPSocket.Port {
			return false
		}
	}
	return true
}

// desiredVolumeMountsPresent checks that all mounts we want are present, ignoring extra k8s-injected ones
func desiredVolumeMountsPresent(existing, desired []corev1.VolumeMount) bool {
	existingMap := make(map[string]string, len(existing))
	for _, m := range existing {
		existingMap[m.Name] = m.MountPath
	}
	for _, d := range desired {
		if existingMap[d.Name] != d.MountPath {
			return false
		}
	}
	return true
}

// desiredVolumesPresent checks that all volumes we want are present, ignoring extra k8s-injected ones
func desiredVolumesPresent(existing, desired []corev1.Volume) bool {
	existingMap := make(map[string]bool, len(existing))
	for _, v := range existing {
		existingMap[v.Name] = true
	}
	for _, d := range desired {
		if !existingMap[d.Name] {
			return false
		}
	}
	return true
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

// setResourcesEqual only compares resource values that were explicitly set in desired.
// If desired has no CPU limit set, we don't compare CPU limits — k8s may have injected one.
func setResourcesEqual(existing, desired corev1.ResourceRequirements) bool {
	for _, r := range []corev1.ResourceName{corev1.ResourceCPU, corev1.ResourceMemory} {
		if dv, ok := desired.Requests[r]; ok {
			ev := existing.Requests[r]
			if ev.Cmp(dv) != 0 {
				return false
			}
		}
		if dv, ok := desired.Limits[r]; ok {
			ev := existing.Limits[r]
			if ev.Cmp(dv) != 0 {
				return false
			}
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
	// For large port sets (e.g. expanded ranges) compare first, last, and count only
	if len(a) > 20 {
		return a[0].Port == b[0].Port &&
			a[len(a)-1].Port == b[len(b)-1].Port &&
			a[0].Protocol == b[0].Protocol
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
