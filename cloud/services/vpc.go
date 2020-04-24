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

func (s *Service) ReconcileVPC() error {
	if len(s.scope.UCloudCluster.Status.Network.VPC.VpcId) > 0 {
		return nil
	}
	s.scope.Info("reconcile VPC")
	// Create VPC
	var finalVpc *vpc.VPCInfo
	vpcSpec := s.scope.UCloudCluster.Spec.Network.VPC
	req := s.vpcClient.NewDescribeVPCRequest()
	req.Region = ucloud.String(s.scope.Region())
	req.ProjectId = ucloud.String(s.scope.ProjectId())
	req.Tag = ucloud.String(s.scope.GroupName())
	if vpcSpec.VpcId != "" {
		req.VPCIds = append(req.VPCIds, vpcSpec.VpcId)
	}
	vpcName := vpcSpec.VpcName
	if vpcName == "" {
		vpcName = "cluster-api-" + s.scope.Name() + "-vpc"
	}
	vpcs, err := s.vpcClient.DescribeVPC(req)
	if err != nil {
		return errors.Errorf("describe vpc failed: %s", err.Error())
	}
	vpcExist := false
	for _, vpcInfo := range vpcs.DataSet {
		if vpcInfo.Name == vpcName || vpcInfo.VPCId == vpcSpec.VpcId {
			finalVpc = &vpcInfo
			vpcExist = true
			break
		}
	}
	if !vpcExist {
		req := s.vpcClient.NewCreateVPCRequest()
		req.Region = ucloud.String(s.scope.Region())
		req.ProjectId = ucloud.String(s.scope.ProjectId())
		req.Name = ucloud.String(vpcName)
		req.Network = append(req.Network, vpcSpec.CidrBlock)
		req.Tag = ucloud.String(s.scope.GroupName())
		vpcInfo, err := s.vpcClient.CreateVPC(req)
		if err != nil {
			return errors.Errorf("create vpc failed: name %s, cidr %s", vpcName, vpcSpec.CidrBlock)
		}
		finalVpc = &vpc.VPCInfo{
			Name:    vpcName,
			Network: []string{vpcSpec.CidrBlock},
			VPCId:   vpcInfo.VPCId,
		}
	}

	s.scope.Info("reconcile VPC success", "status", finalVpc)

	s.scope.UCloudCluster.Status.Network.VPC.CidrBlock = finalVpc.Network[0]
	s.scope.UCloudCluster.Status.Network.VPC.VpcName = finalVpc.Name
	s.scope.UCloudCluster.Status.Network.VPC.VpcId = finalVpc.VPCId
	return nil
}

func (s *Service) DeleteVPC() error {

	s.scope.Info("delete VPC")

	id := s.scope.UCloudCluster.Status.Network.VPC.VpcId
	if len(id) == 0 {
		return nil
	}

	if s.scope.UCloudCluster.Spec.Network.VPC.VpcId == id {
		s.scope.Info("vpc was not created by cluster-api-provider-ucloud, will not be deleted", "vpcid", id)
		return nil
	}
	delReq := s.vpcClient.NewDeleteVPCRequest()
	delReq.Region = ucloud.String(s.scope.Region())
	delReq.ProjectId = ucloud.String(s.scope.ProjectId())
	delReq.VPCId = ucloud.String(id)
	res, err := s.vpcClient.DeleteVPC(delReq)
	if err != nil && res.GetRetCode() != 58103 {
		return errors.Errorf("delete vpc %s failed", id)
	}
	s.scope.Info("delete vpc success", "vpcid", id)
	return nil
}
