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

	// Pod prerequisites must exist before the Deployment is created. A pod that
	// references a ServiceAccount which doesn't exist yet is rejected by the
	// ServiceAccount admission controller ("serviceaccount not found"), and even
	// once it starts it would be missing the RoleBindings that grant it access —
	// so the container comes up without the permissions it was meant to have
	// (e.g. kubectl can't create pods). Create the ServiceAccount/RBAC, PVCs, and
	// any raw resources the pod consumes first. These are best-effort: a failure
	// here is logged but must not block the Deployment.
	if err := EnsureRBAC(ctx, c, app); err != nil {
		log.Error(err, "failed to ensure RBAC (non-fatal)")
	}
	if err := EnsureVolumes(ctx, c, app); err != nil {
		log.Error(err, "failed to ensure volumes (non-fatal)")
	}
	if err := EnsureResources(ctx, c, app); err != nil {
		log.Error(err, "failed to apply generic resources (non-fatal)")
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
					ImagePullSecrets:   imagePullSecrets,
					HostNetwork:        app.Spec.Run.HostNetwork,
					ServiceAccountName: resolveServiceAccountName(app),
					EnableServiceLinks: app.Spec.Run.EnableServiceLinks,
					SecurityContext:    buildPodSecurityContext(app),
					Volumes:           volumes,
					Containers: []corev1.Container{
						{
							Name:            "app",
							Image:           image,
							Command:         nullIfEmpty(app.Spec.Run.Command),
							Args:            nullIfEmpty(app.Spec.Run.Args),
							SecurityContext: buildContainerSecurityContext(app),
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

	// Deployment and service are fatal — app can't run without them.
	// Everything below is best-effort — failures are logged but don't block deployment.
	// (RBAC, volumes, and generic resources are ensured above, before the Deployment.)

	if err := EnsureIngress(ctx, c, app, port); err != nil {
		log.Error(err, "failed to reconcile ingress (non-fatal)")
	}
	if err := EnsureGateway(ctx, c, app, port); err != nil {
		log.Error(err, "failed to reconcile HTTPRoute (non-fatal)")
	}
	if err := EnsureHPA(ctx, c, app); err != nil {
		log.Error(err, "failed to ensure HPA (non-fatal)")
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

func buildPodSecurityContext(app *v1.App) *corev1.PodSecurityContext {
	if app.Spec.Run.SecurityContext == nil {
		return nil
	}
	sc := app.Spec.Run.SecurityContext
	psc := &corev1.PodSecurityContext{}
	if sc.RunAsUser != nil {
		psc.RunAsUser = sc.RunAsUser
	}
	if sc.RunAsGroup != nil {
		psc.RunAsGroup = sc.RunAsGroup
	}
	if sc.RunAsNonRoot != nil {
		psc.RunAsNonRoot = sc.RunAsNonRoot
	}
	if sc.FSGroup != nil {
		psc.FSGroup = sc.FSGroup
	}
	return psc
}

func buildContainerSecurityContext(app *v1.App) *corev1.SecurityContext {
	if app.Spec.Run.ContainerSecurityContext == nil {
		return nil
	}
	sc := app.Spec.Run.ContainerSecurityContext
	csc := &corev1.SecurityContext{}
	if sc.RunAsUser != nil {
		csc.RunAsUser = sc.RunAsUser
	}
	if sc.RunAsGroup != nil {
		csc.RunAsGroup = sc.RunAsGroup
	}
	if sc.RunAsNonRoot != nil {
		csc.RunAsNonRoot = sc.RunAsNonRoot
	}
	if sc.ReadOnlyRootFilesystem != nil {
		csc.ReadOnlyRootFilesystem = sc.ReadOnlyRootFilesystem
	}
	if sc.AllowPrivilegeEscalation != nil {
		csc.AllowPrivilegeEscalation = sc.AllowPrivilegeEscalation
	}
	if sc.Privileged != nil {
		csc.Privileged = sc.Privileged
	}
	if sc.Capabilities != nil {
		caps := &corev1.Capabilities{}
		for _, c := range sc.Capabilities.Add {
			caps.Add = append(caps.Add, corev1.Capability(c))
		}
		for _, c := range sc.Capabilities.Drop {
			caps.Drop = append(caps.Drop, corev1.Capability(c))
		}
		csc.Capabilities = caps
	}
	if sc.SeccompProfile != nil {
		t := corev1.SeccompProfileType(sc.SeccompProfile.Type)
		csc.SeccompProfile = &corev1.SeccompProfile{Type: t}
	}
	return csc
}
