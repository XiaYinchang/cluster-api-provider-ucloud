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
	"github.com/ucloud/ucloud-sdk-go/ucloud"
	infrav1 "sigs.k8s.io/cluster-api-provider-ucloud/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-ucloud/cloud/common"
)

func (s *Service) createEIP(eipSpec infrav1.EIPSpec) (eip infrav1.EIP, err error) {
	s.scope.Info("create eip")
	req := s.unetClient.NewAllocateEIPRequest()
	req.Region = ucloud.String(s.scope.Region())
	req.ProjectId = ucloud.String(s.scope.ProjectId())
	req.Tag = ucloud.String(s.scope.GroupName())
	req.OperatorName = ucloud.String(common.RegionEIPOperator[s.scope.Region()])
	if eipSpec.Bandwidth != 0 {
		req.Bandwidth = ucloud.Int(eipSpec.Bandwidth)
	} else {
		req.Bandwidth = ucloud.Int(common.DefaultNatGatewayEIPBandwidth)
	}
	if eipSpec.EIPName != "" {
		req.Name = ucloud.String(eipSpec.EIPName)
	}

	eipInfo, err := s.unetClient.AllocateEIP(req)
	if err != nil {
		return infrav1.EIP{}, errors.Errorf("allocate eip failed: %s", err.Error())
	}
	newEIP := eipInfo.EIPSet[0]
	eip.Bandwidth = ucloud.IntValue(req.Bandwidth)
	eip.EIPId = newEIP.EIPId
	eip.EIPAddr = newEIP.EIPAddr[0].IP
	eip.EIPName = ucloud.StringValue(req.Name)
	s.scope.Info("create eip success", "eipAddr", newEIP.EIPAddr, "eipId", newEIP.EIPId)
	return eip, nil
}

func (s *Service) deleteEIP(eipId string) error {
	s.scope.Info("delete eip")
	req := s.unetClient.NewReleaseEIPRequest()
	req.Region = ucloud.String(s.scope.Region())
	req.ProjectId = ucloud.String(s.scope.ProjectId())
	req.EIPId = ucloud.String(eipId)
	_, err := s.unetClient.ReleaseEIP(req)
	if err != nil {
		return errors.Errorf("release eip %s failed: %s", eipId, err.Error())
	}
	s.scope.Info("release eip success", "eipId", eipId)
	return nil
}

func (s *Service) bindEIP(eipId, resourceId, resourceType string) error {
	s.scope.Info("start bind eip")
	req := s.unetClient.NewBindEIPRequest()
	req.Region = ucloud.String(s.scope.Region())
	req.ProjectId = ucloud.String(s.scope.ProjectId())
	req.EIPId = ucloud.String(eipId)
	req.ResourceId = ucloud.String(resourceId)
	req.ResourceType = ucloud.String(resourceType)
	_, err := s.unetClient.BindEIP(req)
	if err != nil {
		return errors.Errorf("bind eip %s with %s %s failed: %s", eipId, resourceType, resourceId, err.Error())
	}
	s.scope.Info("bind eip success", "eipId", eipId, "resourceType", resourceType, "resourceId", resourceId)
	return nil
}

func (s *Service) unbindEIP(eipId, resourceId, resourceType string) error {
	s.scope.Info("start unbind eip")
	req := s.unetClient.NewUnBindEIPRequest()
	req.Region = ucloud.String(s.scope.Region())
	req.ProjectId = ucloud.String(s.scope.ProjectId())
	req.EIPId = ucloud.String(eipId)
	req.ResourceId = ucloud.String(resourceId)
	req.ResourceType = ucloud.String(resourceType)
	_, err := s.unetClient.UnBindEIP(req)
	if err != nil {
		return errors.Errorf("unbind eip %s with %s %s failed: %s", eipId, resourceType, resourceId, err.Error())
	}
	s.scope.Info("unbind eip success", "eipId", eipId, "resourceType", resourceType, "resourceId", resourceId)
	return nil
}
