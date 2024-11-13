/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterClassroomSpec defines the desired state of ClusterClassroom
type ClusterClassroomSpec struct {
	NamespacePrefix string  `json:"namespacePrefix,omitempty"`
	Constructor     JobSpec `json:"constructor,omitempty"`
	Evaluator       JobSpec `json:"evaluator,omitempty"`
	StudentId       string  `json:"studentId,omitempty"`
}

// JobSpec defines a Job
type JobSpec struct {
	Image           string            `json:"image,omitempty"`
	Command         []string          `json:"command,omitempty"`
	Args            []string          `json:"args,omitempty"`
	BackoffLimit    int32             `json:"backoffLimit,omitempty"`
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`
}

// ClusterClassroomStatus defines the observed state of ClusterClassroom
type ClusterClassroomStatus struct {
	Namespace           string `json:"namespace,omitempty"`
	NamespacePhase      string `json:"namespacePhase,omitempty"`
	ConstructorJobPhase string `json:"constructorJobPhase,omitempty"`
	EvaluatorJobPhase   string `json:"evaluatorJobPhase,omitempty"`
	ServiceAccount      string `json:"serviceAccount,omitempty"`
	ServiceAccountRole  string `json:"serviceAccountRole,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=cc

// ClusterClassroom is the Schema for the clusterclassrooms API
type ClusterClassroom struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterClassroomSpec   `json:"spec,omitempty"`
	Status ClusterClassroomStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterClassroomList contains a list of ClusterClassroom
type ClusterClassroomList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterClassroom `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterClassroom{}, &ClusterClassroomList{})
}
