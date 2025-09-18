package v1alpha1

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime"
    "k8s.io/apimachinery/pkg/runtime/schema"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=customhpas,scope=Namespaced,shortName=chpa
// +kubebuilder:printcolumn:name="Min",type=integer,JSONPath=`.spec.minReplicas`
// +kubebuilder:printcolumn:name="Max",type=integer,JSONPath=`.spec.maxReplicas`
// +kubebuilder:printcolumn:name="Target",type=string,JSONPath=`.spec.targetRef.name`
// +kubebuilder:printcolumn:name="Replicas",type=integer,JSONPath=`.status.currentReplicas`
type CustomHPA struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   CustomHPASpec   `json:"spec,omitempty"`
    Status CustomHPAStatus `json:"status,omitempty"`
}

type ObjectRef struct {
    // Name of the target resource (Deployment only for this PoC)
    Name string `json:"name"`
    // Namespace of the target resource. If empty, defaults to this resource namespace.
    Namespace string `json:"namespace,omitempty"`
}

type CustomHPASpec struct {
    // Minimum replicas to keep.
    MinReplicas int32 `json:"minReplicas"`
    // Maximum replicas allowed.
    MaxReplicas int32 `json:"maxReplicas"`
    // Poll interval in seconds (default 30)
    IntervalSeconds *int32 `json:"intervalSeconds,omitempty"`
    // Target deployment reference.
    TargetRef ObjectRef `json:"targetRef"`
}

type CustomHPAStatus struct {
    // Current replicas observed on the target Deployment.
    CurrentReplicas int32 `json:"currentReplicas,omitempty"`
    // Last time we performed a scale action.
    LastScaleTime *metav1.Time `json:"lastScaleTime,omitempty"`
    // Conditions for readiness and errors.
    Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
type CustomHPAList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`
    Items           []CustomHPA `json:"items"`
}

func init() {}

// AddToScheme registers types to the scheme.
func AddToScheme(s *runtime.Scheme) error {
    gvk := GroupVersion
    s.AddKnownTypes(gvk,
        &CustomHPA{},
        &CustomHPAList{},
    )
    metav1.AddToGroupVersion(s, gvk)
    return nil
}

var GroupVersion = schema.GroupVersion{Group: "monitoring.pisanix.dev", Version: "v1alpha1"}

// DeepCopy methods (sem codegen) para satisfazer runtime.Object

func (in *CustomHPA) DeepCopyInto(out *CustomHPA) {
    *out = *in
    out.TypeMeta = in.TypeMeta
    in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
    out.Spec = in.Spec
    // Deep copy de Status
    out.Status.CurrentReplicas = in.Status.CurrentReplicas
    if in.Status.LastScaleTime != nil {
        t := *in.Status.LastScaleTime
        out.Status.LastScaleTime = &t
    } else {
        out.Status.LastScaleTime = nil
    }
    if in.Status.Conditions != nil {
        out.Status.Conditions = make([]metav1.Condition, len(in.Status.Conditions))
        copy(out.Status.Conditions, in.Status.Conditions)
    } else {
        out.Status.Conditions = nil
    }
}

func (in *CustomHPA) DeepCopy() *CustomHPA {
    if in == nil { return nil }
    out := new(CustomHPA)
    in.DeepCopyInto(out)
    return out
}

func (in *CustomHPA) DeepCopyObject() runtime.Object {
    if c := in.DeepCopy(); c != nil { return c }
    return nil
}

func (in *CustomHPAList) DeepCopyInto(out *CustomHPAList) {
    *out = *in
    out.TypeMeta = in.TypeMeta
    in.ListMeta.DeepCopyInto(&out.ListMeta)
    if in.Items != nil {
        out.Items = make([]CustomHPA, len(in.Items))
        for i := range in.Items {
            in.Items[i].DeepCopyInto(&out.Items[i])
        }
    } else {
        out.Items = nil
    }
}

func (in *CustomHPAList) DeepCopy() *CustomHPAList {
    if in == nil { return nil }
    out := new(CustomHPAList)
    in.DeepCopyInto(out)
    return out
}

func (in *CustomHPAList) DeepCopyObject() runtime.Object {
    if c := in.DeepCopy(); c != nil { return c }
    return nil
}
