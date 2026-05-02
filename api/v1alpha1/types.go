package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var GroupVersion = schema.GroupVersion{
	Group:   "kube-deploy.centerionware.app",
	Version: "v1alpha1",
}

// ----------------------------------------------------------------
// App — build from source and deploy
// Formerly NpmApp. Supports any language/build system.
// ----------------------------------------------------------------

type App struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AppSpec   `json:"spec,omitempty"`
	Status AppStatus `json:"status,omitempty"`
}

type AppList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []App `json:"items"`
}

func (in *App) DeepCopyObject() runtime.Object {
	out := new(App)
	*out = *in
	return out
}

func (in *AppList) DeepCopyObject() runtime.Object {
	out := new(AppList)
	*out = *in
	return out
}

type AppSpec struct {
	// Repo is the git repository URL to clone and build
	Repo string `json:"repo"`

	// UpdateInterval controls how often git is polled for new commits.
	// Accepts Go duration strings: 30s, 1m, 5m, 1h. Defaults to 1m.
	UpdateInterval string `json:"updateInterval,omitempty"`

	Env map[string]string `json:"env,omitempty"`

	Build   BuildSpec       `json:"build,omitempty"`
	Run     RunSpec         `json:"run,omitempty"`
	Service ServiceSpec     `json:"service,omitempty"`
	Ingress *IngressSpec    `json:"ingress,omitempty"`
	Gateway *GatewaySpec    `json:"gateway,omitempty"`
}

// ----------------------------------------------------------------
// ContainerApp — deploy a pre-built image, no build stage
// ----------------------------------------------------------------

type ContainerApp struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ContainerAppSpec   `json:"spec,omitempty"`
	Status ContainerAppStatus `json:"status,omitempty"`
}

type ContainerAppList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ContainerApp `json:"items"`
}

func (in *ContainerApp) DeepCopyObject() runtime.Object {
	out := new(ContainerApp)
	*out = *in
	return out
}

func (in *ContainerAppList) DeepCopyObject() runtime.Object {
	out := new(ContainerAppList)
	*out = *in
	return out
}

type ContainerAppSpec struct {
	// Image is the full image ref to deploy e.g. "nginx:latest"
	Image string `json:"image"`

	Env     map[string]string `json:"env,omitempty"`
	Run     RunSpec           `json:"run,omitempty"`
	Service ServiceSpec       `json:"service,omitempty"`
	Ingress *IngressSpec      `json:"ingress,omitempty"`
	Gateway *GatewaySpec      `json:"gateway,omitempty"`
}

type ContainerAppStatus struct {
	// Message holds human-readable status or error detail
	Message    string `json:"message,omitempty"`
	Phase      string `json:"phase,omitempty"`
	LastUpdate string `json:"lastUpdate,omitempty"`
}

// ----------------------------------------------------------------
// BUILD
// ----------------------------------------------------------------

type BuildSpec struct {
	// BaseImage is the builder base image. Default: node:20-alpine
	BaseImage string `json:"baseImage,omitempty"`

	// InstallCmd runs first. Default: npm install --legacy-peer-deps
	InstallCmd string `json:"installCmd,omitempty"`

	// BuildCmd runs after install. Default: npm run build
	BuildCmd string `json:"buildCmd,omitempty"`

	// Output overrides the full image name (without tag)
	Output string   `json:"output,omitempty"`
	Args   []string `json:"args,omitempty"`

	// Registry to push built images to — reachable from buildkitd
	// Default: registry.registry.svc.cluster.local:5000
	Registry string `json:"registry,omitempty"`

	// GitSecret: k8s Secret name for git auth. Omit for public repos.
	// HTTPS: "username" + "password". SSH: "ssh-privatekey" + optional "ssh-passphrase".
	GitSecret string `json:"gitSecret,omitempty"`

	// RegistrySecret: k8s Secret name for registry push auth (future)
	RegistrySecret string `json:"registrySecret,omitempty"`
}

// ----------------------------------------------------------------
// RUN — shared by App and ContainerApp
// ----------------------------------------------------------------

type RunSpec struct {
	Command  []string `json:"command,omitempty"`
	Args     []string `json:"args,omitempty"`
	Port     int      `json:"port,omitempty"`
	Replicas int      `json:"replicas,omitempty"`

	// Registry to pull images from — reachable from containerd on nodes
	// Default: localhost:31999
	Registry string `json:"registry,omitempty"`

	// ImagePullSecret: k8s Secret name for private registry pull auth
	ImagePullSecret string `json:"imagePullSecret,omitempty"`

	Resources   ResourceSpec     `json:"resources,omitempty"`
	HealthCheck HealthCheckSpec  `json:"healthCheck,omitempty"`
	Volumes     []VolumeSpec     `json:"volumes,omitempty"`
	Autoscaling *AutoscalingSpec `json:"autoscaling,omitempty"`
}

type ResourceSpec struct {
	CPURequest    string `json:"cpuRequest,omitempty"`
	MemoryRequest string `json:"memoryRequest,omitempty"`
	CPULimit      string `json:"cpuLimit,omitempty"`
	MemoryLimit   string `json:"memoryLimit,omitempty"`
}

type HealthCheckSpec struct {
	// Path for HTTP liveness+readiness e.g. "/healthz"
	// Falls back to TCP socket check if empty
	Path string `json:"path,omitempty"`
}

type VolumeSpec struct {
	Name         string `json:"name"`
	MountPath    string `json:"mountPath"`
	Size         string `json:"size,omitempty"`
	StorageClass string `json:"storageClass,omitempty"`
}

type AutoscalingSpec struct {
	Enabled     bool `json:"enabled"`
	MinReplicas int  `json:"minReplicas,omitempty"`
	MaxReplicas int  `json:"maxReplicas,omitempty"`
	// CPUTarget is target CPU utilization percentage. Default: 80
	CPUTarget int `json:"cpuTarget,omitempty"`
}

// ----------------------------------------------------------------
// SERVICE — full Kubernetes Service override surface
// ----------------------------------------------------------------

type ServiceSpec struct {
	// Type: ClusterIP | NodePort | LoadBalancer | ExternalName. Default: ClusterIP
	Type string `json:"type,omitempty"`

	Annotations map[string]string `json:"annotations,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`

	// ClusterIP override. Use "None" for headless.
	ClusterIP string `json:"clusterIP,omitempty"`

	ExternalIPs              []string `json:"externalIPs,omitempty"`
	LoadBalancerIP           string   `json:"loadBalancerIP,omitempty"`
	LoadBalancerSourceRanges []string `json:"loadBalancerSourceRanges,omitempty"`

	// ExternalTrafficPolicy: Cluster | Local
	ExternalTrafficPolicy string `json:"externalTrafficPolicy,omitempty"`

	// SessionAffinity: None | ClientIP
	SessionAffinity string `json:"sessionAffinity,omitempty"`

	PublishNotReadyAddresses bool `json:"publishNotReadyAddresses,omitempty"`

	// Ports overrides the default single-port mapping from run.port
	Ports []ServicePortSpec `json:"ports,omitempty"`
}

type ServicePortSpec struct {
	Name       string `json:"name,omitempty"`
	Port       int32  `json:"port"`
	TargetPort int32  `json:"targetPort,omitempty"`
	NodePort   int32  `json:"nodePort,omitempty"`
	// Protocol: TCP | UDP. Default: TCP
	Protocol string `json:"protocol,omitempty"`
}

// ----------------------------------------------------------------
// INGRESS
// ----------------------------------------------------------------

type IngressSpec struct {
	Enabled   bool    `json:"enabled"`
	Host      string  `json:"host,omitempty"`
	ClassName *string `json:"className,omitempty"`
	TLSSecret string  `json:"tlsSecret,omitempty"`

	Annotations map[string]string `json:"annotations,omitempty"`

	// Paths overrides the default "/" PathPrefix route
	Paths []IngressPathSpec `json:"paths,omitempty"`
}

type IngressPathSpec struct {
	Path string `json:"path"`
	// PathType: Prefix | Exact | ImplementationSpecific. Default: Prefix
	PathType string `json:"pathType,omitempty"`
}

// ----------------------------------------------------------------
// GATEWAY API — HTTPRoute
// Use either ingress or gateway, not both.
// ----------------------------------------------------------------

type GatewaySpec struct {
	Enabled bool `json:"enabled"`

	// GatewayRef is the Gateway this HTTPRoute attaches to
	GatewayRef GatewayRefSpec `json:"gatewayRef"`

	// Hostnames the HTTPRoute matches on
	Hostnames []string `json:"hostnames,omitempty"`

	// TLSSecret for cert reference in the Gateway listener (optional)
	TLSSecret string `json:"tlsSecret,omitempty"`

	// Paths overrides the default "/" PathPrefix match
	Paths []GatewayPathSpec `json:"paths,omitempty"`

	Annotations map[string]string `json:"annotations,omitempty"`
}

type GatewayRefSpec struct {
	Name      string `json:"name"`
	// Namespace defaults to same namespace as the App
	Namespace   string `json:"namespace,omitempty"`
	// SectionName targets a specific listener on the Gateway
	SectionName string `json:"sectionName,omitempty"`
}

type GatewayPathSpec struct {
	Path string `json:"path"`
	// MatchType: PathPrefix | Exact | RegularExpression. Default: PathPrefix
	MatchType string `json:"matchType,omitempty"`
}

// ----------------------------------------------------------------
// STATUS — shared shape
// ----------------------------------------------------------------

type AppStatus struct {
	Phase      string `json:"phase,omitempty"`
	Image      string `json:"image,omitempty"`
	Commit     string `json:"commit,omitempty"`
	LastUpdate string `json:"lastUpdate,omitempty"`
}
