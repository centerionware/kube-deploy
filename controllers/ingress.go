package controllers

import (
	"context"

	v1 "kube-deploy/api/v1alpha1"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func EnsureIngress(ctx context.Context, c client.Client, app *v1.App, port int32) error {
	log := log.FromContext(ctx).WithValues("app", app.Name, "namespace", app.Namespace)

	if app.Spec.Ingress == nil || !app.Spec.Ingress.Enabled {
		var existing networkingv1.Ingress
		if err := c.Get(ctx, client.ObjectKey{Name: app.Name, Namespace: app.Namespace}, &existing); err == nil {
			log.Info("removing ingress (disabled)")
			return c.Delete(ctx, &existing)
		}
		return nil
	}

	paths := app.Spec.Ingress.Paths
	if len(paths) == 0 {
		paths = []v1.IngressPathSpec{{Path: "/", PathType: "Prefix"}}
	}

	var httpPaths []networkingv1.HTTPIngressPath
	for _, p := range paths {
		pt := networkingv1.PathTypePrefix
		switch p.PathType {
		case "Exact":
			pt = networkingv1.PathTypeExact
		case "ImplementationSpecific":
			pt = networkingv1.PathTypeImplementationSpecific
		}
		pathCopy := p.Path
		httpPaths = append(httpPaths, networkingv1.HTTPIngressPath{
			Path:     pathCopy,
			PathType: &pt,
			Backend: networkingv1.IngressBackend{
				Service: &networkingv1.IngressServiceBackend{
					Name: app.Name,
					Port: networkingv1.ServiceBackendPort{Number: port},
				},
			},
		})
	}

	ing := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        app.Name,
			Namespace:   app.Namespace,
			Annotations: app.Spec.Ingress.Annotations,
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: app.Spec.Ingress.ClassName,
			Rules: []networkingv1.IngressRule{
				{
					Host: app.Spec.Ingress.Host,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{Paths: httpPaths},
					},
				},
			},
		},
	}

	if app.Spec.Ingress.TLSSecret != "" {
		ing.Spec.TLS = []networkingv1.IngressTLS{
			{Hosts: []string{app.Spec.Ingress.Host}, SecretName: app.Spec.Ingress.TLSSecret},
		}
	}

	var existing networkingv1.Ingress
	err := c.Get(ctx, client.ObjectKeyFromObject(&ing), &existing)
	if errors.IsNotFound(err) {
		log.Info("creating ingress", "host", app.Spec.Ingress.Host)
		return c.Create(ctx, &ing)
	}
	if err != nil {
		return err
	}
	log.Info("updating ingress", "host", app.Spec.Ingress.Host)
	ing.ResourceVersion = existing.ResourceVersion
	return c.Update(ctx, &ing)
}
