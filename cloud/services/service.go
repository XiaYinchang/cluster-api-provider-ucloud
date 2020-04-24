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

package services

import (
	"github.com/ucloud/ucloud-sdk-go/services/udisk"
	"github.com/ucloud/ucloud-sdk-go/services/uhost"
	"github.com/ucloud/ucloud-sdk-go/services/ulb"
	"github.com/ucloud/ucloud-sdk-go/services/unet"
	"github.com/ucloud/ucloud-sdk-go/services/uphost"
	"github.com/ucloud/ucloud-sdk-go/services/vpc"

	"sigs.k8s.io/cluster-api-provider-ucloud/cloud/scope"
)

// Service holds a collection of interfaces.
// The interfaces are broken down like this to group functions together.
// One alternative is to have a large list of functions from the ucloud client.
type Service struct {
	scope *scope.ClusterScope

	// Helper clients for GCP.
	uhostClient  *uhost.UHostClient
	unetClient   *unet.UNetClient
	vpcClient    *vpc.VPCClient
	ulbClient    *ulb.ULBClient
	udiskClient  *udisk.UDiskClient
	uphostClient *uphost.UPHostClient
}

// NewService returns a new service given the ucloud api client.
func NewService(newScope *scope.ClusterScope) *Service {
	return &Service{
		scope:        newScope,
		uhostClient:  uhost.NewClient(newScope.Config, newScope.Credential),
		unetClient:   unet.NewClient(newScope.Config, newScope.Credential),
		vpcClient:    vpc.NewClient(newScope.Config, newScope.Credential),
		ulbClient:    ulb.NewClient(newScope.Config, newScope.Credential),
		udiskClient:  udisk.NewClient(newScope.Config, newScope.Credential),
		uphostClient: uphost.NewClient(newScope.Config, newScope.Credential),
	}
}
