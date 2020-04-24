package services

import (
	"time"

	"github.com/pkg/errors"
	"github.com/ucloud/ucloud-sdk-go/ucloud"
	"github.com/ucloud/ucloud-sdk-go/ucloud/request"
	"github.com/ucloud/ucloud-sdk-go/ucloud/response"
	"sigs.k8s.io/cluster-api-provider-ucloud/cloud/scope"
)

func (s *Service) CreateCAPUCluster() error {
	if s.scope.UCloudCluster.Status.ClusterId != "" {
		return nil
	}
	req := &CreateCAPUClusterRequest{}
	req.SetAction("CreateCAPUCluster")
	req.SetRequestTime(time.Now())
	req.Region = ucloud.String(s.scope.Region())
	req.ProjectId = ucloud.String(s.scope.ProjectId())
	req.VPCId = ucloud.String(s.scope.UCloudCluster.Status.Network.VPC.VpcId)
	req.SubnetId = ucloud.String(s.scope.UCloudCluster.Status.Network.Subnet.SubnetId)
	req.PodCIDR = ucloud.String(s.scope.Cluster.Spec.ClusterNetwork.Pods.CIDRBlocks[0])
	req.NATGWId = ucloud.String(s.scope.UCloudCluster.Status.Network.Nat.NatGatewayId)
	req.ServiceCIDR = ucloud.String(s.scope.Cluster.Spec.ClusterNetwork.Services.CIDRBlocks[0])
	req.ClusterName = ucloud.String(s.scope.UCloudCluster.Name)
	req.ULBId = ucloud.String(s.scope.UCloudCluster.Status.Network.ULB.LoadBalancerId)
	req.FWId = ucloud.String(s.scope.UCloudCluster.Status.Network.Firewall.FirewallId)
	req.NodeCIDR = ucloud.String(s.scope.UCloudCluster.Status.Network.Subnet.CidrBlock)
	req.K8SVersion = ucloud.String(s.scope.UCloudCluster.Spec.Version)
	req.APIServer = ucloud.String(s.scope.UCloudCluster.Spec.ControlPlaneEndpoint.String())
	bastionInfo := s.scope.UCloudCluster.Status.Bastion
	if bastionInfo != nil {
		req.BastionId = ucloud.String(bastionInfo.InstanceId)
		req.BastionZone = ucloud.String(bastionInfo.Zone)
	}

	var res CreateCAPUClusterResponse
	err := s.doRequest(req, &res)
	if err != nil {
		return errors.Wrap(err, "create uk8s capu cluster failed")
	}
	s.scope.UCloudCluster.Status.ClusterId = res.ClusterId
	return nil
}

func (s *Service) DeleteCAPUCluster() error {
	if s.scope.UCloudCluster.Status.ClusterId == "" {
		return nil
	}
	req := &DeleteCAPUClusterRequest{}
	req.SetAction("DeleteCAPUCluster")
	req.SetRequestTime(time.Now())
	req.Region = ucloud.String(s.scope.Region())
	req.ProjectId = ucloud.String(s.scope.ProjectId())
	req.ClusterId = ucloud.String(s.scope.UCloudCluster.Status.ClusterId)

	var res DeleteCAPUClusterResponse
	err := s.doRequest(req, &res)
	if err != nil {
		return errors.Wrap(err, "delete uk8s capu cluster failed")
	}
	return nil
}

func (s *Service) CreateCAPUHost(scope *scope.MachineScope) error {
	if scope.UCloudMachine.Status.ClusterId != "" {
		return nil
	}
	req := &CreateCAPUHostRequest{}
	req.SetAction("CreateCAPUHost")
	req.SetRequestTime(time.Now())
	req.Region = ucloud.String(s.scope.Region())
	req.ProjectId = ucloud.String(s.scope.ProjectId())
	req.ClusterId = ucloud.String(s.scope.UCloudCluster.Status.ClusterId)
	req.InstanceId = scope.GetInstanceID()
	req.Zone = ucloud.String(scope.GetZone())
	req.Type = ucloud.String("uhost")
	if scope.IsControlPlane() {
		req.Role = ucloud.String("master")
	} else {
		req.Role = ucloud.String("node")
	}

	var res CreateCAPUHostResponse
	err := s.doRequest(req, &res)
	if err != nil {
		return errors.Wrap(err, "create uk8s capu host failed")
	}
	scope.UCloudMachine.Status.ClusterId = s.scope.UCloudCluster.Status.ClusterId
	return nil
}

func (s *Service) DeleteCAPUHost(scope *scope.MachineScope) error {
	req := &DeleteCAPUHostRequest{}
	req.SetAction("DeleteCAPUHost")
	req.SetRequestTime(time.Now())
	req.Region = ucloud.String(s.scope.Region())
	req.ProjectId = ucloud.String(s.scope.ProjectId())
	req.ClusterId = ucloud.String(s.scope.UCloudCluster.Status.ClusterId)
	req.InstanceId = scope.GetInstanceID()

	var res DeleteCAPUHostResponse
	err := s.doRequest(req, &res)
	if err != nil {
		return errors.Wrap(err, "delete uk8s capu host failed")
	}
	return nil
}

type CreateCAPUClusterRequest struct {
	request.CommonBase
	VPCId       *string `required:"true"`
	SubnetId    *string `required:"true"`
	PodCIDR     *string `required:"true"`
	NATGWId     *string `required:"true"`
	ServiceCIDR *string `required:"true"`
	ClusterName *string `required:"true"`
	ULBId       *string `required:"true"`
	FWId        *string `required:"true"`
	NodeCIDR    *string `required:"false"`
	K8SVersion  *string `required:"true"`
	APIServer   *string `required:"true"`
	BastionId   *string `required:"false"`
	BastionZone *string `required:"false"`
}

// CreateCAPUClusterResponse is response schema for CreateCAPUCluster action
type CreateCAPUClusterResponse struct {
	response.CommonBase
	ClusterId string
}

// DeleteCAPUClusterRequest
type DeleteCAPUClusterRequest struct {
	request.CommonBase
	ClusterId *string `required:"true"`
}

// DeleteCAPUClusterResponse
type DeleteCAPUClusterResponse struct {
	response.CommonBase
}

// CreateCAPUHostRequest
type CreateCAPUHostRequest struct {
	request.CommonBase
	ClusterId  *string `required:"true"`
	InstanceId *string `required:"true"`
	Type       *string `required:"true"`
	Role       *string `required:"true"`
}

// CreateCAPUHostResponse
type CreateCAPUHostResponse struct {
	response.CommonBase
}

// DeleteCAPUHostRequest
type DeleteCAPUHostRequest struct {
	request.CommonBase
	ClusterId  *string `required:"true"`
	InstanceId *string `required:"true"`
}

// DeleteCAPUHostResponse
type DeleteCAPUHostResponse struct {
	response.CommonBase
}
