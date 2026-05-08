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
	Repo           string            `json:"repo"`
	UpdateInterval string            `json:"updateInterval,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	Build          BuildSpec         `json:"build,omitempty"`
	Run            RunSpec           `json:"run,omitempty"`
	Service        ServiceSpec       `json:"service,omitempty"`
	Ingress        *IngressSpec      `json:"ingress,omitempty"`
	Gateway        *GatewaySpec      `json:"gateway,omitempty"`
	RBAC           *RBACSpec         `json:"rbac,omitempty"`
}

// ----------------------------------------------------------------
// ContainerApp — deploy a pre-built image
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
	Image   string            `json:"image"`
	Env     map[string]string `json:"env,omitempty"`
	Run     RunSpec           `json:"run,omitempty"`
	Service ServiceSpec       `json:"service,omitempty"`
	Ingress *IngressSpec      `json:"ingress,omitempty"`
	Gateway *GatewaySpec      `json:"gateway,omitempty"`
	RBAC    *RBACSpec         `json:"rbac,omitempty"`
}

type ContainerAppStatus struct {
	Phase      string `json:"phase,omitempty"`
	Message    string `json:"message,omitempty"`
	LastUpdate string `json:"lastUpdate,omitempty"`
}

// ----------------------------------------------------------------
// BUILD
// ----------------------------------------------------------------

type BuildSpec struct {
	BaseImage      string   `json:"baseImage,omitempty"`
	InstallCmd     string   `json:"installCmd,omitempty"`
	BuildCmd       string   `json:"buildCmd,omitempty"`

	// DockerfileMode controls how the Dockerfile is sourced:
	//   auto     (default) — use repo Dockerfile if present, else generate
	//   generate — always use the built-in generator
	//   inline   — use the Dockerfile provided in the Dockerfile field
	DockerfileMode string `json:"dockerfileMode,omitempty"`

	// Dockerfile contains a complete Dockerfile, used only when DockerfileMode is "inline"
	Dockerfile string `json:"dockerfile,omitempty"`
	Output         string   `json:"output,omitempty"`
	Args           []string `json:"args,omitempty"`
	Registry       string   `json:"registry,omitempty"`
	GitSecret      string   `json:"gitSecret,omitempty"`
	RegistrySecret string   `json:"registrySecret,omitempty"`
}

// ----------------------------------------------------------------
// RUN — shared by App and ContainerApp
// ----------------------------------------------------------------

type RunSpec struct {
	Command         []string         `json:"command,omitempty"`
	Args            []string         `json:"args,omitempty"`
	Port            int              `json:"port,omitempty"`
	Replicas        int              `json:"replicas,omitempty"`
	Registry        string           `json:"registry,omitempty"`
	ImagePullSecret  string   `json:"imagePullSecret,omitempty"`
	// ImagePullSecrets is a list of Secret names for pulling from multiple private registries
	ImagePullSecrets []string `json:"imagePullSecrets,omitempty"`
	HostNetwork     bool             `json:"hostNetwork,omitempty"`
	// ServiceAccountName to use for the pod. If rbac is set, defaults to app name.
	ServiceAccountName string          `json:"serviceAccountName,omitempty"`
	Resources       ResourceSpec     `json:"resources,omitempty"`
	HealthCheck     HealthCheckSpec  `json:"healthCheck,omitempty"`
	Volumes         []VolumeSpec     `json:"volumes,omitempty"`
	Autoscaling     *AutoscalingSpec `json:"autoscaling,omitempty"`
}

type ResourceSpec struct {
	CPURequest    string `json:"cpuRequest,omitempty"`
	MemoryRequest string `json:"memoryRequest,omitempty"`
	CPULimit      string `json:"cpuLimit,omitempty"`
	MemoryLimit   string `json:"memoryLimit,omitempty"`
}

type HealthCheckSpec struct {
	Path string `json:"path,omitempty"`
}

// VolumeSpec supports PVC, ConfigMap, Secret, EmptyDir, and HostPath.
// Exactly one source field should be set.
type VolumeSpec struct {
	// Name is the volume name, referenced by the mount
	Name      string `json:"name"`
	MountPath string `json:"mountPath"`

	// --- Source types (set exactly one) ---

	// PVC creates or mounts a PersistentVolumeClaim
	PVC *PVCVolumeSource `json:"pvc,omitempty"`

	// ConfigMap mounts a ConfigMap as a volume
	ConfigMap *ConfigMapVolumeSource `json:"configMap,omitempty"`

	// Secret mounts a Secret as a volume
	Secret *SecretVolumeSource `json:"secret,omitempty"`

	// EmptyDir mounts a temporary empty directory
	EmptyDir *EmptyDirVolumeSource `json:"emptyDir,omitempty"`

	// HostPath mounts a path from the host node
	HostPath *HostPathVolumeSource `json:"hostPath,omitempty"`
}

type PVCVolumeSource struct {
	// ClaimName of an existing PVC, or leave empty to auto-create one
	ClaimName    string `json:"claimName,omitempty"`
	Size         string `json:"size,omitempty"`
	StorageClass string `json:"storageClass,omitempty"`
	ReadOnly     bool   `json:"readOnly,omitempty"`
}

type ConfigMapVolumeSource struct {
	// Name of the ConfigMap
	Name string `json:"name"`
	// Optional: mount specific keys as files
	// If empty, all keys are mounted
	Items []KeyToPath `json:"items,omitempty"`
}

type SecretVolumeSource struct {
	// Name of the Secret
	SecretName string `json:"secretName"`
	// Optional: mount specific keys as files
	Items []KeyToPath `json:"items,omitempty"`
}

type EmptyDirVolumeSource struct {
	// Medium: "" (default) or "Memory" for tmpfs
	Medium string `json:"medium,omitempty"`
}

type HostPathVolumeSource struct {
	Path string `json:"path"`
	// Type: "", DirectoryOrCreate, Directory, FileOrCreate, File, Socket, etc.
	Type string `json:"type,omitempty"`
}

type KeyToPath struct {
	// Key in the ConfigMap or Secret
	Key  string `json:"key"`
	// Path to mount the key as (filename inside the mountPath)
	Path string `json:"path"`
}

type AutoscalingSpec struct {
	Enabled     bool `json:"enabled"`
	MinReplicas int  `json:"minReplicas,omitempty"`
	MaxReplicas int  `json:"maxReplicas,omitempty"`
	CPUTarget   int  `json:"cpuTarget,omitempty"`
}

// ----------------------------------------------------------------
// RBAC
// ----------------------------------------------------------------

// RBACSpec defines the full RBAC chain for an app.
// The operator creates a ServiceAccount and wires it into the pod.
type RBACSpec struct {
	// ServiceAccountName overrides the auto-generated name (defaults to app name)
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// Roles defines namespace-scoped roles to CREATE and bind to the ServiceAccount
	Roles []RoleDefinition `json:"roles,omitempty"`

	// ClusterRoles defines cluster-scoped roles to CREATE and bind to the ServiceAccount
	ClusterRoles []RoleDefinition `json:"clusterRoles,omitempty"`

	// RoleBindings binds EXISTING namespace Roles to the ServiceAccount (by name)
	RoleBindings []string `json:"roleBindings,omitempty"`

	// ClusterRoleBindings binds EXISTING ClusterRoles to the ServiceAccount (by name)
	// e.g. ["cluster-admin", "view"]
	ClusterRoleBindings []string `json:"clusterRoleBindings,omitempty"`
}

// RoleDefinition defines a Role or ClusterRole to be created
type RoleDefinition struct {
	// Name of the Role/ClusterRole to create
	Name  string     `json:"name"`
	Rules []RBACRule `json:"rules"`
}

type RBACRule struct {
	// APIGroups e.g. [""] for core, ["apps"], ["batch"]
	APIGroups []string `json:"apiGroups"`
	Resources []string `json:"resources"`
	Verbs     []string `json:"verbs"`
	// ResourceNames restricts to specific named resources (optional)
	ResourceNames []string `json:"resourceNames,omitempty"`
}

// ----------------------------------------------------------------
// SERVICE
// ----------------------------------------------------------------

type ServiceSpec struct {
	Type                     string            `json:"type,omitempty"`
	Annotations              map[string]string `json:"annotations,omitempty"`
	Labels                   map[string]string `json:"labels,omitempty"`
	ClusterIP                string            `json:"clusterIP,omitempty"`
	ExternalIPs              []string          `json:"externalIPs,omitempty"`
	LoadBalancerIP           string            `json:"loadBalancerIP,omitempty"`
	LoadBalancerSourceRanges []string          `json:"loadBalancerSourceRanges,omitempty"`
	ExternalTrafficPolicy    string            `json:"externalTrafficPolicy,omitempty"`
	SessionAffinity          string            `json:"sessionAffinity,omitempty"`
	PublishNotReadyAddresses bool              `json:"publishNotReadyAddresses,omitempty"`
	Ports                    []ServicePortSpec  `json:"ports,omitempty"`
	// PortRanges expands a range of ports into individual service ports
	PortRanges               []PortRangeSpec    `json:"portRanges,omitempty"`
}

type ServicePortSpec struct {
	Name       string `json:"name,omitempty"`
	Port       int32  `json:"port"`
	TargetPort int32  `json:"targetPort,omitempty"`
	NodePort   int32  `json:"nodePort,omitempty"`
	Protocol   string `json:"protocol,omitempty"`
}

// PortRangeSpec expands a range of ports into individual service ports.
// e.g. start: 50000, end: 60000, protocol: UDP
type PortRangeSpec struct {
	// Start of the port range (inclusive)
	Start int32 `json:"start"`
	// End of the port range (inclusive)
	End int32 `json:"end"`
	// Protocol: TCP or UDP. Default: UDP
	Protocol string `json:"protocol,omitempty"`
	// TargetPortOffset allows the target port to differ from the service port.
	// target = port + offset. Default: 0 (same as service port)
	TargetPortOffset int32 `json:"targetPortOffset,omitempty"`
}

// ----------------------------------------------------------------
// INGRESS
// ----------------------------------------------------------------

type IngressSpec struct {
	Enabled     bool              `json:"enabled"`
	Host        string            `json:"host,omitempty"`
	ClassName   *string           `json:"className,omitempty"`
	TLSSecret   string            `json:"tlsSecret,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Paths       []IngressPathSpec `json:"paths,omitempty"`
}

type IngressPathSpec struct {
	Path     string `json:"path"`
	PathType string `json:"pathType,omitempty"`
}

// ----------------------------------------------------------------
// GATEWAY API
// ----------------------------------------------------------------

type GatewaySpec struct {
	Enabled     bool              `json:"enabled"`
	GatewayRef  GatewayRefSpec    `json:"gatewayRef"`
	Hostnames   []string          `json:"hostnames,omitempty"`
	TLSSecret   string            `json:"tlsSecret,omitempty"`
	Paths       []GatewayPathSpec `json:"paths,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type GatewayRefSpec struct {
	Name        string `json:"name"`
	Namespace   string `json:"namespace,omitempty"`
	SectionName string `json:"sectionName,omitempty"`
}

type GatewayPathSpec struct {
	Path      string `json:"path"`
	MatchType string `json:"matchType,omitempty"`
}

// ----------------------------------------------------------------
// STATUS
// ----------------------------------------------------------------

type AppStatus struct {
	Phase      string `json:"phase,omitempty"`
	Image      string `json:"image,omitempty"`
	Commit     string `json:"commit,omitempty"`
	LastUpdate string `json:"lastUpdate,omitempty"`
}
