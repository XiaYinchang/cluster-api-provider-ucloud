/*
Copyright 2019 The Kubernetes Authors.

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

package scope

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"k8s.io/klog/klogr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-ucloud/api/v1alpha3"
)

// ClusterScopeParams defines the input parameters used to create a new Scope.
type ClusterScopeParams struct {
	UCloudClients
	Client        client.Client
	Logger        logr.Logger
	Cluster       *clusterv1.Cluster
	UCloudCluster *infrav1.UCloudCluster
}

// NewClusterScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewClusterScope(params ClusterScopeParams) (*ClusterScope, error) {
	if params.Cluster == nil {
		return nil, errors.New("failed to generate new scope from nil Cluster")
	}
	if params.UCloudCluster == nil {
		return nil, errors.New("failed to generate new scope from nil UCloudCluster")
	}

	if params.Logger == nil {
		params.Logger = klogr.New()
	}

	if params.UCloudClients.Config == nil {
		params.UCloudClients.loadDefaultConfig()
	}

	if params.UCloudClients.Credential == nil {
		params.UCloudClients.getCredentialFromEnv()
	}

	helper, err := patch.NewHelper(params.UCloudCluster, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}
	return &ClusterScope{
		Logger:        params.Logger,
		client:        params.Client,
		UCloudClients: params.UCloudClients,
		Cluster:       params.Cluster,
		UCloudCluster: params.UCloudCluster,
		patchHelper:   helper,
	}, nil
}

// ClusterScope defines the basic context for an actuator to operate upon.
type ClusterScope struct {
	logr.Logger
	client      client.Client
	patchHelper *patch.Helper

	UCloudClients
	Cluster       *clusterv1.Cluster
	UCloudCluster *infrav1.UCloudCluster
}

// Project returns the current project name.
func (s *ClusterScope) ProjectId() string {
	return s.UCloudCluster.Spec.ProjectId
}

// VPCName returns the cluster network unique identifier.
func (s *ClusterScope) VPCName() string {
	if s.UCloudCluster.Spec.Network.VPC.VpcName != "" {
		return s.UCloudCluster.Spec.Network.VPC.VpcName
	}
	return "default"
}

// Network returns the cluster network object.
func (s *ClusterScope) Network() *infrav1.Network {
	return &s.UCloudCluster.Status.Network
}

// GroupName
func (s *ClusterScope) GroupName() string {
	return s.UCloudCluster.Status.Group.GroupName
}

// Subnets returns the cluster subnets.
func (s *ClusterScope) Subnet() infrav1.SubnetSpec {
	return s.UCloudCluster.Spec.Network.Subnet
}

// Name returns the cluster name.
func (s *ClusterScope) Name() string {
	return s.Cluster.Name
}

// Namespace returns the cluster namespace.
func (s *ClusterScope) Namespace() string {
	return s.Cluster.Namespace
}

// Region returns the cluster region.
func (s *ClusterScope) Region() string {
	return s.UCloudCluster.Spec.Region
}

// LoadBalancerFrontendPort returns the loadbalancer frontend if specified
// in the cluster resource's network configuration.
func (s *ClusterScope) LoadBalancerFrontendPort() int64 {
	if s.Cluster.Spec.ClusterNetwork.APIServerPort != nil {
		return int64(*s.Cluster.Spec.ClusterNetwork.APIServerPort)
	}
	return 6443
}

// LoadBalancerBackendPort returns the loadbalancer backend if specified.
func (s *ClusterScope) LoadBalancerBackendPort() int64 {
	return 6443
}

// ControlPlaneConfigMapName returns the name of the ConfigMap used to
// coordinate the bootstrapping of control plane nodes.
func (s *ClusterScope) ControlPlaneConfigMapName() string {
	return fmt.Sprintf("%s-controlplane", s.Cluster.UID)
}

// ListOptionsLabelSelector returns a ListOptions with a label selector for clusterName.
func (s *ClusterScope) ListOptionsLabelSelector() client.ListOption {
	return client.MatchingLabels(map[string]string{
		clusterv1.ClusterLabelName: s.Cluster.Name,
	})
}

// PatchObject persists the cluster configuration and status.
func (s *ClusterScope) PatchObject() error {
	return s.patchHelper.Patch(context.TODO(), s.UCloudCluster)
}

// Close closes the current scope persisting the cluster configuration and status.
func (s *ClusterScope) Close() error {
	return s.PatchObject()
}
