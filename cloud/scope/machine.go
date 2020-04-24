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

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/klogr"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/controllers/noderefutil"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-ucloud/api/v1alpha3"
)

// MachineScopeParams defines the input parameters used to create a new MachineScope.
type MachineScopeParams struct {
	Client        client.Client
	Logger        logr.Logger
	Cluster       *clusterv1.Cluster
	Machine       *clusterv1.Machine
	UCloudCluster *infrav1.UCloudCluster
	UCloudMachine *infrav1.UCloudMachine
}

// NewMachineScope creates a new MachineScope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewMachineScope(params MachineScopeParams) (*MachineScope, error) {
	if params.Client == nil {
		return nil, errors.New("client is required when creating a MachineScope")
	}
	if params.Machine == nil {
		return nil, errors.New("machine is required when creating a MachineScope")
	}
	if params.Cluster == nil {
		return nil, errors.New("cluster is required when creating a MachineScope")
	}
	if params.UCloudCluster == nil {
		return nil, errors.New("ucloud cluster is required when creating a MachineScope")
	}
	if params.UCloudMachine == nil {
		return nil, errors.New("ucloud machine is required when creating a MachineScope")
	}

	if params.Logger == nil {
		params.Logger = klogr.New()
	}

	helper, err := patch.NewHelper(params.UCloudMachine, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}
	return &MachineScope{
		client:        params.Client,
		Cluster:       params.Cluster,
		Machine:       params.Machine,
		UCloudCluster: params.UCloudCluster,
		UCloudMachine: params.UCloudMachine,
		Logger:        params.Logger,
		patchHelper:   helper,
	}, nil
}

// MachineScope defines a scope defined around a machine and its cluster.
type MachineScope struct {
	logr.Logger
	client      client.Client
	patchHelper *patch.Helper

	Cluster       *clusterv1.Cluster
	Machine       *clusterv1.Machine
	UCloudCluster *infrav1.UCloudCluster
	UCloudMachine *infrav1.UCloudMachine
}

// Region returns the UCloudMachine region.
func (m *MachineScope) Region() string {
	return m.UCloudCluster.Spec.Region
}

// Zone returns the FailureDomain for the UCloudMachine.
func (m *MachineScope) Zone() string {
	if m.Machine.Spec.FailureDomain == nil {
		return m.UCloudMachine.Status.Zone
	}
	return *m.Machine.Spec.FailureDomain
}

// Name returns the UCloudMachine name.
func (m *MachineScope) Name() string {
	return m.UCloudMachine.Name
}

// Namespace returns the namespace name.
func (m *MachineScope) Namespace() string {
	return m.UCloudMachine.Namespace
}

// IsControlPlane returns true if the machine is a control plane.
func (m *MachineScope) IsControlPlane() bool {
	return util.IsControlPlaneMachine(m.Machine)
}

// Role returns the machine role from the labels.
func (m *MachineScope) Role() string {
	if util.IsControlPlaneMachine(m.Machine) {
		return "control-plane"
	}
	return "node"
}

// GetInstanceID returns the UCloudMachine instance id by parsing Spec.ProviderID.
func (m *MachineScope) GetInstanceID() *string {
	parsed, err := noderefutil.NewProviderID(m.GetProviderID())
	if err != nil {
		return nil
	}
	return pointer.StringPtr(parsed.ID())
}

// GetProviderID returns the UCloudMachine providerID from the spec.
func (m *MachineScope) GetProviderID() string {
	if m.UCloudMachine.Spec.ProviderID != nil {
		return *m.UCloudMachine.Spec.ProviderID
	}
	return ""
}

// SetProviderID sets the UCloudMachine providerID in spec.
func (m *MachineScope) SetProviderID(v string) {
	m.UCloudMachine.Spec.ProviderID = pointer.StringPtr(v)
}

// GetInstanceStatus returns the UCloudMachine instance status.
func (m *MachineScope) GetInstanceStatus() string {
	return m.UCloudMachine.Status.InstanceStatus
}

// SetInstanceStatus sets the UCloudMachine instance status.
func (m *MachineScope) SetInstanceStatus(v string) {
	m.UCloudMachine.Status.InstanceStatus = v
}

// SetReady sets the UCloudMachine Ready Status
func (m *MachineScope) SetReady() {
	m.UCloudMachine.Status.Ready = true
}

// SetNotReady sets the UCloudMachine Not Ready Status
func (m *MachineScope) SetNotReady() {
	m.UCloudMachine.Status.Ready = false
}

// SetZone sets the UCloudMachine Zone in status
func (m *MachineScope) SetZone(v string) {
	m.UCloudMachine.Status.Zone = v
}

// GetZone sets the UCloudMachine Zone in status
func (m *MachineScope) GetZone() string {
	return m.UCloudMachine.Status.Zone
}

// SetFailureMessage sets the UCloudMachine status failure message.
func (m *MachineScope) SetFailureMessage(v error) {
	m.UCloudMachine.Status.FailureMessage = pointer.StringPtr(v.Error())
}

// SetFailureReason sets the UCloudMachine status failure reason.
func (m *MachineScope) SetFailureReason(v capierrors.MachineStatusError) {
	m.UCloudMachine.Status.FailureReason = &v
}

// SetAnnotation sets a key value annotation on the UCloudMachine.
func (m *MachineScope) SetAnnotation(key, value string) {
	if m.UCloudMachine.Annotations == nil {
		m.UCloudMachine.Annotations = map[string]string{}
	}
	m.UCloudMachine.Annotations[key] = value
}

// SetAddresses sets the addresses field on the UCloudMachine.
func (m *MachineScope) SetAddresses(addressList []clusterv1.MachineAddress) {
	m.UCloudMachine.Status.Addresses = addressList
}

// GetBootstrapData returns the bootstrap data from the secret in the Machine's bootstrap.dataSecretName.
func (m *MachineScope) GetBootstrapData() (string, error) {
	if m.Machine.Spec.Bootstrap.DataSecretName == nil {
		return "", errors.New("error retrieving bootstrap data: linked Machine's bootstrap.dataSecretName is nil")
	}

	secret := &corev1.Secret{}
	key := types.NamespacedName{Namespace: m.Namespace(), Name: *m.Machine.Spec.Bootstrap.DataSecretName}
	if err := m.client.Get(context.TODO(), key, secret); err != nil {
		return "", errors.Wrapf(err, "failed to retrieve bootstrap data secret for UCloudMachine %s/%s", m.Namespace(), m.Name())
	}

	value, ok := secret.Data["value"]
	if !ok {
		return "", errors.New("error retrieving bootstrap data: secret value key is missing")
	}
	return string(value), nil
}

// PatchObject persists the cluster configuration and status.
func (m *MachineScope) PatchObject() error {
	return m.patchHelper.Patch(context.TODO(), m.UCloudMachine)
}

// Close closes the current scope persisting the cluster configuration and status.
func (m *MachineScope) Close() error {
	return m.PatchObject()
}
