package controllers

import (
	"context"
	"fmt"
	"strings"

	v1 "kube-deploy/api/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const defaultPort = 3000

func EnsureRuntime(ctx context.Context, c client.Client, app *v1.App, image string) error {
	log := log.FromContext(ctx).WithValues("app", app.Name, "namespace", app.Namespace)

	port := int32(app.Spec.Run.Port)
	if port == 0 {
		port = defaultPort
	}

	replicas := int32(1)
	if app.Spec.Run.Replicas > 0 {
		replicas = int32(app.Spec.Run.Replicas)
	}

	podLabels := map[string]string{
		"app":                   app.Name,
		"kube-deploy/app":       app.Name,
		"kube-deploy/namespace": app.Namespace,
	}

	volumes, volumeMounts, err := buildVolumes(app.Spec.Run.Volumes)
	if err != nil {
		log.Error(err, "failed to build volumes")
		return err
	}

	resources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    must("50m"),
			corev1.ResourceMemory: must("64Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    must("500m"),
			corev1.ResourceMemory: must("512Mi"),
		},
	}
	if app.Spec.Run.Resources.CPURequest != "" {
		resources.Requests[corev1.ResourceCPU] = must(app.Spec.Run.Resources.CPURequest)
	}
	if app.Spec.Run.Resources.MemoryRequest != "" {
		resources.Requests[corev1.ResourceMemory] = must(app.Spec.Run.Resources.MemoryRequest)
	}
	if app.Spec.Run.Resources.CPULimit != "" {
		resources.Limits[corev1.ResourceCPU] = must(app.Spec.Run.Resources.CPULimit)
	}
	if app.Spec.Run.Resources.MemoryLimit != "" {
		resources.Limits[corev1.ResourceMemory] = must(app.Spec.Run.Resources.MemoryLimit)
	}

	var liveness, readiness *corev1.Probe
	if app.Spec.Run.HealthCheck.Path != "" {
		liveness = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: app.Spec.Run.HealthCheck.Path,
					Port: intstr.FromInt32(port),
				},
			},
			InitialDelaySeconds: 10,
			PeriodSeconds:       15,
			FailureThreshold:    3,
		}
		readiness = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: app.Spec.Run.HealthCheck.Path,
					Port: intstr.FromInt32(port),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
			FailureThreshold:    3,
		}
	} else {
		readiness = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt32(port),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
		}
	}

	// Merge ImagePullSecret (singular) and ImagePullSecrets (plural) into one list
	var imagePullSecrets []corev1.LocalObjectReference
	if app.Spec.Run.ImagePullSecret != "" {
		imagePullSecrets = append(imagePullSecrets, corev1.LocalObjectReference{Name: app.Spec.Run.ImagePullSecret})
	}
	for _, s := range app.Spec.Run.ImagePullSecrets {
		imagePullSecrets = append(imagePullSecrets, corev1.LocalObjectReference{Name: s})
	}

	log.Info("upserting deployment", "image", image, "port", port, "replicas", replicas)
	deploy := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,
			Labels:    podLabels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": app.Name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: podLabels},
				Spec: corev1.PodSpec{
					ImagePullSecrets:  imagePullSecrets,
					HostNetwork:       app.Spec.Run.HostNetwork,
					ServiceAccountName: resolveServiceAccountName(app),
					Volumes:           volumes,
					Containers: []corev1.Container{
						{
							Name:           "app",
							Image:          image,
							Command:        app.Spec.Run.Command,
							Args:           app.Spec.Run.Args,
							Env:            buildEnv(app.Spec.Env),
							Resources:      resources,
							VolumeMounts:   volumeMounts,
							LivenessProbe:  liveness,
							ReadinessProbe: readiness,
							Ports: []corev1.ContainerPort{
								{ContainerPort: port, Protocol: corev1.ProtocolTCP},
							},
						},
					},
				},
			},
		},
	}

	if err := upsertDeployment(ctx, c, deploy); err != nil {
		log.Error(err, "failed to upsert deployment")
		return err
	}
	log.Info("deployment upserted", "image", image)

	svc := buildService(app, port)
	if err := upsertService(ctx, c, svc); err != nil {
		log.Error(err, "failed to upsert service")
		return err
	}
	log.Info("service upserted", "type", svc.Spec.Type)

	// Always call both — each handles its own cleanup when disabled.
	// This ensures switching between ingress and gateway cleans up the other.
	if err := EnsureIngress(ctx, c, app, port); err != nil {
		log.Error(err, "failed to reconcile ingress")
		return err
	}
	if err := EnsureGateway(ctx, c, app, port); err != nil {
		log.Error(err, "failed to reconcile HTTPRoute")
		return err
	}

	if err := EnsureRBAC(ctx, c, app); err != nil {
		log.Error(err, "failed to ensure service account")
		return err
	}

	if err := EnsureVolumes(ctx, c, app); err != nil {
		log.Error(err, "failed to ensure volumes")
		return err
	}

	if err := EnsureHPA(ctx, c, app); err != nil {
		log.Error(err, "failed to ensure HPA")
		return err
	}
    if err := EnsureResources(ctx, c, app); err != nil {
        log.Error(err, "failed to apply generic resources")
        return err
    }
	return nil
}

func buildService(app *v1.App, defaultPort int32) corev1.Service {
	spec := app.Spec.Service

	var svcPorts []corev1.ServicePort

	// Explicit named ports
	for _, p := range spec.Ports {
		proto := corev1.ProtocolTCP
		if p.Protocol == "UDP" {
			proto = corev1.ProtocolUDP
		}
		targetPort := p.TargetPort
		if targetPort == 0 {
			targetPort = p.Port
		}
		sp := corev1.ServicePort{
			Name:       p.Name,
			Port:       p.Port,
			TargetPort: intstr.FromInt32(targetPort),
			Protocol:   proto,
		}
		if p.NodePort != 0 {
			sp.NodePort = p.NodePort
		}
		svcPorts = append(svcPorts, sp)
	}

	// Port ranges — expanded into individual ports
	for _, r := range spec.PortRanges {
		proto := corev1.ProtocolUDP
		if r.Protocol == "TCP" {
			proto = corev1.ProtocolTCP
		}
		for port := r.Start; port <= r.End; port++ {
			targetPort := port + r.TargetPortOffset
			svcPorts = append(svcPorts, corev1.ServicePort{
				Name:       fmt.Sprintf("%s-%d", strings.ToLower(string(proto)), port),
				Port:       port,
				TargetPort: intstr.FromInt32(targetPort),
				Protocol:   proto,
			})
		}
	}

	// Default single port if nothing specified
	if len(svcPorts) == 0 {
		svcPorts = []corev1.ServicePort{
			{
				Port:       defaultPort,
				TargetPort: intstr.FromInt32(defaultPort),
				Protocol:   corev1.ProtocolTCP,
			},
		}
	}

	svcType := corev1.ServiceTypeClusterIP
	if spec.Type != "" {
		svcType = corev1.ServiceType(spec.Type)
	}

	svcLabels := map[string]string{
		"kube-deploy/app":       app.Name,
		"kube-deploy/namespace": app.Namespace,
	}
	for k, v := range spec.Labels {
		svcLabels[k] = v
	}

	svcSpec := corev1.ServiceSpec{
		Type:     svcType,
		Selector: map[string]string{"app": app.Name},
		Ports:    svcPorts,
	}
	if spec.ClusterIP != "" {
		svcSpec.ClusterIP = spec.ClusterIP
	}
	if len(spec.ExternalIPs) > 0 {
		svcSpec.ExternalIPs = spec.ExternalIPs
	}
	if spec.LoadBalancerIP != "" {
		svcSpec.LoadBalancerIP = spec.LoadBalancerIP
	}
	if len(spec.LoadBalancerSourceRanges) > 0 {
		svcSpec.LoadBalancerSourceRanges = spec.LoadBalancerSourceRanges
	}
	if spec.ExternalTrafficPolicy != "" {
		svcSpec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicy(spec.ExternalTrafficPolicy)
	}
	if spec.SessionAffinity != "" {
		svcSpec.SessionAffinity = corev1.ServiceAffinity(spec.SessionAffinity)
	}
	if spec.PublishNotReadyAddresses {
		svcSpec.PublishNotReadyAddresses = true
	}

	return corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        app.Name,
			Namespace:   app.Namespace,
			Labels:      svcLabels,
			Annotations: spec.Annotations,
		},
		Spec: svcSpec,
	}
}

// resolveServiceAccountName returns the service account name for the pod.
func resolveServiceAccountName(app *v1.App) string {
	if app.Spec.Run.ServiceAccountName != "" {
		return app.Spec.Run.ServiceAccountName
	}
	if app.Spec.RBAC != nil {
		if app.Spec.RBAC.ServiceAccountName != "" {
			return app.Spec.RBAC.ServiceAccountName
		}
		return app.Name // auto-created SA named after the app
	}
	return "" // use default SA
}
