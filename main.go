package main

import (
	"log"
	"os"

	v1 "kube-deploy/api/v1alpha1"
	"kube-deploy/controllers"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1.AddToScheme(scheme))
	utilruntime.Must(appsv1.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(batchv1.AddToScheme(scheme))
	utilruntime.Must(networkingv1.AddToScheme(scheme))
	utilruntime.Must(autoscalingv2.AddToScheme(scheme))
	// Gateway API — registered separately, non-fatal if CRDs not installed
	if err := gatewayv1.Install(scheme); err != nil {
		log.Printf("warning: gateway API scheme registration failed (CRDs may not be installed): %v", err)
	}
}

func main() {
	zapOpts := zap.Options{
		Development: os.Getenv("LOG_DEV_MODE") != "false",
	}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zapOpts)))

	cfg := ctrl.GetConfigOrDie()

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme,
	})
	if err != nil {
		log.Printf("manager init failed: %v", err)
		os.Exit(1)
	}

	// App reconciler — build from source and deploy
	if err := controllers.Setup(mgr, &controllers.AppReconciler{
		Client: mgr.GetClient(),
	}); err != nil {
		log.Printf("App controller setup failed: %v", err)
		os.Exit(1)
	}

	// ContainerApp reconciler — deploy pre-built images directly
	if err := controllers.SetupContainerApp(mgr, &controllers.ContainerAppReconciler{
		Client: mgr.GetClient(),
	}); err != nil {
		log.Printf("ContainerApp controller setup failed: %v", err)
		os.Exit(1)
	}

	log.Println("starting kube-deploy controller manager...")

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Printf("manager exited: %v", err)
		os.Exit(1)
	}
}
