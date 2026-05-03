package controllers

import (
	"context"
	"fmt"

	v1 "kube-deploy/api/v1alpha1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func EnsureGateway(ctx context.Context, c client.Client, app *v1.App, port int32) error {
	log := log.FromContext(ctx).WithValues("app", app.Name, "namespace", app.Namespace)

	if app.Spec.Gateway == nil || !app.Spec.Gateway.Enabled {
		if err := deleteHTTPRoute(ctx, c, app.Name, app.Namespace); err != nil && !errors.IsNotFound(err) {
			log.Error(err, "failed to remove HTTPRoute")
		}
		return nil
	}

	gw := app.Spec.Gateway
	gwNamespace := gw.GatewayRef.Namespace
	if gwNamespace == "" {
		gwNamespace = app.Namespace
	}

	parentRef := gatewayv1.ParentReference{
		Name:      gatewayv1.ObjectName(gw.GatewayRef.Name),
		Namespace: (*gatewayv1.Namespace)(&gwNamespace),
	}
	if gw.GatewayRef.SectionName != "" {
		sn := gatewayv1.SectionName(gw.GatewayRef.SectionName)
		parentRef.SectionName = &sn
	}

	var hostnames []gatewayv1.Hostname
	for _, h := range gw.Hostnames {
		hostnames = append(hostnames, gatewayv1.Hostname(h))
	}

	paths := gw.Paths
	if len(paths) == 0 {
		paths = []v1.GatewayPathSpec{{Path: "/", MatchType: "PathPrefix"}}
	}

	var rules []gatewayv1.HTTPRouteRule
	for _, p := range paths {
		matchType := gatewayv1.PathMatchPathPrefix
		switch p.MatchType {
		case "Exact":
			matchType = gatewayv1.PathMatchExact
		case "RegularExpression":
			matchType = gatewayv1.PathMatchRegularExpression
		}
		pathVal := p.Path
		rules = append(rules, gatewayv1.HTTPRouteRule{
			Matches: []gatewayv1.HTTPRouteMatch{
				{
					Path: &gatewayv1.HTTPPathMatch{
						Type:  &matchType,
						Value: &pathVal,
					},
				},
			},
			BackendRefs: []gatewayv1.HTTPBackendRef{
				{
					BackendRef: gatewayv1.BackendRef{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: gatewayv1.ObjectName(app.Name),
							Port: (*gatewayv1.PortNumber)(&port),
						},
					},
				},
			},
		})
	}

	route := gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:        app.Name,
			Namespace:   app.Namespace,
			Annotations: gw.Annotations,
		},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{parentRef},
			},
			Hostnames: hostnames,
			Rules:     rules,
		},
	}

	var existing gatewayv1.HTTPRoute
	err := c.Get(ctx, client.ObjectKeyFromObject(&route), &existing)
	if errors.IsNotFound(err) {
		log.Info("creating HTTPRoute",
			"gateway", fmt.Sprintf("%s/%s", gwNamespace, gw.GatewayRef.Name),
			"hostnames", gw.Hostnames,
		)
		return c.Create(ctx, &route)
	}
	if err != nil {
		return err
	}
	log.Info("updating HTTPRoute", "gateway", gw.GatewayRef.Name)
	route.ResourceVersion = existing.ResourceVersion
	return c.Update(ctx, &route)
}

func deleteHTTPRoute(ctx context.Context, c client.Client, name, namespace string) error {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "gateway.networking.k8s.io",
		Version: "v1",
		Kind:    "HTTPRoute",
	})
	obj.SetName(name)
	obj.SetNamespace(namespace)
	err := c.Delete(ctx, obj)
	if errors.IsNotFound(err) {
		return nil
	}
	return err
}
