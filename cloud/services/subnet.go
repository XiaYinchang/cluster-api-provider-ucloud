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
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/ucloud/ucloud-sdk-go/services/vpc"
	"github.com/ucloud/ucloud-sdk-go/ucloud"
)

func (s *Service) ReconcileSubnet() error {
	if len(s.scope.UCloudCluster.Status.Network.Subnet.SubnetId) > 0 {
		return nil
	}
	vpcId := s.scope.UCloudCluster.Status.Network.VPC.VpcId
	if vpcId == "" {
		return errors.Errorf("vpc is not created, subnet must be owned by a vpc")
	}
	s.scope.Info("reconcile subnet")
	// Create Subnet
	var finalSubnet *vpc.VPCSubnetInfoSet
	subnetSpec := s.scope.UCloudCluster.Spec.Network.Subnet
	req := s.vpcClient.NewDescribeSubnetRequest()
	req.Region = ucloud.String(s.scope.Region())
	req.ProjectId = ucloud.String(s.scope.ProjectId())
	req.VPCId = ucloud.String(vpcId)
	req.Tag = ucloud.String(s.scope.GroupName())
	if subnetSpec.SubnetId != "" {
		req.SubnetId = ucloud.String(subnetSpec.SubnetId)
	}
	subnetName := subnetSpec.SubnetName
	if subnetName == "" {
		subnetName = "cluster-api-" + s.scope.Cluster.GetName() + "-subnet"
	}
	subnets, err := s.vpcClient.DescribeSubnet(req)
	if err != nil {
		return errors.Errorf("describe subnet failed: %s", err.Error())
	}
	subnetExist := false
	for _, subnetInfo := range subnets.DataSet {
		if subnetInfo.SubnetName == subnetName || subnetInfo.SubnetId == subnetSpec.SubnetId {
			finalSubnet = &subnetInfo
			subnetExist = true
			break
		}
	}
	if !subnetExist {
		req := s.vpcClient.NewCreateSubnetRequest()
		req.Region = ucloud.String(s.scope.Region())
		req.ProjectId = ucloud.String(s.scope.ProjectId())
		req.SubnetName = ucloud.String(subnetName)
		subnetCidr := strings.Split(subnetSpec.CidrBlock, "/")
		req.Subnet = ucloud.String(subnetCidr[0])
		netmask, _ := strconv.Atoi(subnetCidr[1])
		req.Netmask = ucloud.Int(netmask)
		req.VPCId = ucloud.String(vpcId)
		req.Tag = ucloud.String(s.scope.GroupName())
		subnetInfo, err := s.vpcClient.CreateSubnet(req)
		if err != nil {
			return errors.Errorf("create subnet failed: name %s, cidr %s", subnetName, subnetSpec.CidrBlock)
		}
		finalSubnet = &vpc.VPCSubnetInfoSet{
			SubnetName: subnetName,
			Subnet:     subnetCidr[0],
			Netmask:    subnetCidr[1],
			SubnetId:   subnetInfo.SubnetId,
		}
	}

	s.scope.Info("reconcile subnet success", "status", finalSubnet)

	s.scope.UCloudCluster.Status.Network.Subnet.CidrBlock = finalSubnet.Subnet + "/" + finalSubnet.Netmask
	s.scope.UCloudCluster.Status.Network.Subnet.SubnetName = finalSubnet.SubnetName
	s.scope.UCloudCluster.Status.Network.Subnet.SubnetId = finalSubnet.SubnetId
	s.scope.UCloudCluster.Status.Network.Subnet.VpcId = vpcId
	return nil
}

func (s *Service) DeleteSubnet() error {

	s.scope.Info("delete subnet")

	id := s.scope.UCloudCluster.Status.Network.Subnet.SubnetId
	if len(id) == 0 {
		return nil
	}

	if s.scope.UCloudCluster.Spec.Network.Subnet.SubnetId == id {
		s.scope.Info("subnet was not created by cluster-api-provider-ucloud, will not be deleted", "subnetid", id)
		return nil
	}

	// timer := time.NewTimer(5 * time.Minute)
	// func() {
	// 	for {
	// 		s.scope.Info("waiting for a few minutes because of the delay on UCloud when cleanning resources in subnet")
	// 		select {
	// 		case <-timer.C:
	// 			return
	// 		default:
	// 			time.Sleep(30 * time.Second)
	// 		}
	// 	}
	// }()

	delReq := s.vpcClient.NewDeleteSubnetRequest()
	delReq.Region = ucloud.String(s.scope.Region())
	delReq.ProjectId = ucloud.String(s.scope.ProjectId())
	delReq.SubnetId = ucloud.String(id)
	res, err := s.vpcClient.DeleteSubnet(delReq)
	if err != nil && res.GetRetCode() != 8039 {
		return errors.Errorf("delete subnet %s failed", id)
	}
	s.scope.Info("delete subnet success", "subnetid", id)
	return nil
}
