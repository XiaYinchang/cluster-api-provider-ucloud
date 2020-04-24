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
	"github.com/pkg/errors"
	"github.com/ucloud/ucloud-sdk-go/services/vpc"
	"github.com/ucloud/ucloud-sdk-go/ucloud"
)

func (s *Service) ReconcileNat() error {
	if len(s.scope.UCloudCluster.Status.Network.Nat.NatGatewayId) > 0 {
		return nil
	}
	s.scope.Info("reconcile nat")
	natSpec := s.scope.UCloudCluster.Spec.Network.Nat
	vpcId := s.scope.UCloudCluster.Status.Network.VPC.VpcId
	if vpcId == "" {
		return errors.Errorf("vpc is not created")
	}
	req := s.vpcClient.NewDescribeNATGWRequest()
	req.Region = ucloud.String(s.scope.Region())
	req.ProjectId = ucloud.String(s.scope.ProjectId())
	if natSpec.NatGateway.NatGatewayId != "" {
		req.NATGWIds = append(req.NATGWIds, natSpec.NatGateway.NatGatewayId)
	}
	var finalNatGW *vpc.NatGatewayDataSet
	natGWs, err := s.vpcClient.DescribeNATGW(req)
	if err != nil {
		return errors.Errorf("describe nat gateway failed: %s", err.Error())
	}
	natGWExist := false
	for _, natGW := range natGWs.DataSet {
		if (natGW.NATGWId == natSpec.NatGateway.NatGatewayId || natGW.NATGWName == natSpec.NatGateway.Name) && natGW.VPCId == vpcId {
			natGWExist = true
			finalNatGW = &natGW
			break
		}
	}
	if !natGWExist {
		subnetId := s.scope.UCloudCluster.Status.Network.Subnet.SubnetId
		if subnetId == "" {
			return errors.Errorf("subnet is not created")
		}
		firewall, err := s.getFirewall(s.scope.UCloudCluster.Spec.Network.Firewall.FirewallId)
		if err != nil {
			return err
		}
		firewallId := firewall.FirewallId
		eip, err := s.createEIP(natSpec.EIP)
		if err != nil {
			return err
		}

		// Create Nat
		req := s.vpcClient.NewCreateNATGWRequest()
		req.Region = ucloud.String(s.scope.Region())
		req.ProjectId = ucloud.String(s.scope.ProjectId())
		req.Tag = ucloud.String(s.scope.GroupName())
		if natSpec.NatGateway.Name != "" {
			req.NATGWName = ucloud.String(natSpec.NatGateway.Name)
		} else {
			req.NATGWName = ucloud.String("natgw-for-" + s.scope.UCloudCluster.Name)
		}
		req.FirewallId = ucloud.String(firewallId)
		req.VPCId = ucloud.String(vpcId)
		req.SubnetworkIds = append(req.SubnetworkIds, subnetId)
		req.EIPIds = append(req.EIPIds, eip.EIPId)
		newNat, err := s.vpcClient.CreateNATGW(req)
		if err != nil {
			return errors.Errorf("create nat gw failed: %s", err.Error())
		}
		finalNatGW = &vpc.NatGatewayDataSet{
			FirewallId: firewallId,
			NATGWId:    newNat.NATGWId,
			NATGWName:  ucloud.StringValue(req.NATGWName),
			VPCId:      vpcId,
		}
		finalNatGW.IPSet = append(finalNatGW.IPSet, vpc.NatGatewayIPSet{
			Bandwidth: eip.Bandwidth,
			EIPId:     eip.EIPId,
		})
		finalNatGW.SubnetSet = append(finalNatGW.SubnetSet, vpc.NatGatewaySubnetSet{
			SubnetworkId: subnetId,
		})
	}

	s.scope.Info("reconcile nat success", "status", finalNatGW)

	s.scope.UCloudCluster.Status.Network.Firewall.FirewallId = finalNatGW.FirewallId
	s.scope.UCloudCluster.Status.Network.Nat.NatGatewayId = finalNatGW.NATGWId
	s.scope.UCloudCluster.Status.Network.Nat.Name = finalNatGW.NATGWName
	s.scope.UCloudCluster.Status.Network.Nat.VpcId = finalNatGW.VPCId
	s.scope.UCloudCluster.Status.Network.Nat.Firewall.FirewallId = finalNatGW.FirewallId
	s.scope.UCloudCluster.Status.Network.Nat.EIP.EIPId = finalNatGW.IPSet[0].EIPId
	s.scope.UCloudCluster.Status.Network.Nat.EIP.Bandwidth = finalNatGW.IPSet[0].Bandwidth
	return nil
}

func (s *Service) DeleteNat() error {

	s.scope.Info("delete nat")

	id := s.scope.UCloudCluster.Status.Network.Nat.NatGatewayId
	if len(id) == 0 {
		return nil
	}
	if s.scope.UCloudCluster.Spec.Network.Nat.NatGateway.NatGatewayId == id {
		s.scope.Info("nat gateway was not created by cluster-api-provider-ucloud, will not be deleted", "natgatewayid", id)
		return nil
	}
	// delete natgw
	delReq := s.vpcClient.NewDeleteNATGWRequest()
	delReq.Region = ucloud.String(s.scope.Region())
	delReq.ProjectId = ucloud.String(s.scope.ProjectId())
	delReq.NATGWId = ucloud.String(id)
	delReq.ReleaseEip = ucloud.Bool(true)
	res, err := s.vpcClient.DeleteNATGW(delReq)
	if err != nil && res.GetRetCode() != 54002 {
		return errors.Errorf("delete natgateway %s failed: %s", id, err.Error())
	}
	s.scope.Info("delete nat success", "natgatewayid", id)
	return nil
}
