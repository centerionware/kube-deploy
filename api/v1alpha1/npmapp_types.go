package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// NpmApp is your platform CRD
type NpmApp struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NpmAppSpec   `json:"spec"`
	Status NpmAppStatus `json:"status,omitempty"`
}

type NpmAppSpec struct {
	Repo     string `json:"repo"`
	Revision string `json:"revision,omitempty"`
	Image    string `json:"image"`

	Build NpmBuildSpec `json:"build,omitempty"`
	Run   NpmRunSpec   `json:"run,omitempty"`

	Env map[string]string `json:"env,omitempty"`
}

type NpmBuildSpec struct {
	Builder        string `json:"builder,omitempty"`
	InstallCommand string `json:"installCommand,omitempty"`
	BuildCommand   string `json:"buildCommand,omitempty"`
}

type NpmRunSpec struct {
	Command string `json:"command,omitempty"`
	Port    int    `json:"port,omitempty"`
}

// Status is what makes this production-grade
type NpmAppStatus struct {
	ObservedGeneration int64  `json:"observedGeneration,omitempty"`
	Image              string `json:"image,omitempty"`
	Phase              string `json:"phase,omitempty"` // Pending / Building / Ready / Failed
}