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
	"time"

	"github.com/pkg/errors"
	"github.com/ucloud/ucloud-sdk-go/services/ulb"
	"github.com/ucloud/ucloud-sdk-go/ucloud"
	"github.com/ucloud/ucloud-sdk-go/ucloud/request"
)

func (s *Service) ReconcileULB() error {
	if len(s.scope.UCloudCluster.Status.Network.ULB.LoadBalancerId) > 0 {
		return nil
	}
	s.scope.Info("reconcile ulb")
	ulbSpec := s.scope.UCloudCluster.Spec.Network.ULB
	req := s.ulbClient.NewDescribeULBRequest()
	req.Region = ucloud.String(s.scope.Region())
	req.ProjectId = ucloud.String(s.scope.ProjectId())
	vpcId := s.scope.UCloudCluster.Status.Network.VPC.VpcId
	if vpcId == "" {
		return errors.Errorf("vpc is not created")
	}
	req.VPCId = ucloud.String(vpcId)
	if ulbSpec.LoadBalancerId != "" {
		req.ULBId = ucloud.String(ulbSpec.LoadBalancerId)
	}
	var finalULB *ulb.ULBSet
	ulbs, err := s.ulbClient.DescribeULB(req)
	if err != nil {
		return errors.Errorf("describe ulb failed: %s", err.Error())
	}
	ulbExist := false
	for _, ulb := range ulbs.DataSet {
		if (ulb.Name == ulbSpec.LoadBalancerName || ulb.ULBId == ulbSpec.LoadBalancerId) && ulb.VPCId == vpcId {
			ulbExist = true
			finalULB = &ulb
			break
		}
	}

	if !ulbExist {
		// create ulb
		// req := s.ulbClient.NewCreateULBRequest()
		req := &CreateULBRequestPlus{}
		req.SetAction("CreateULB")
		req.SetRequestTime(time.Now())
		req.Region = ucloud.String(s.scope.Region())
		req.ProjectId = ucloud.String(s.scope.ProjectId())
		req.VPCId = ucloud.String(vpcId)
		req.ListenType = ucloud.String("RequestProxy")
		req.Tag = ucloud.String(s.scope.GroupName())

		if ulbSpec.LoadBalancerName != "" {
			req.ULBName = ucloud.String(ulbSpec.LoadBalancerName)
		} else {
			req.ULBName = ucloud.String("ulb-for-" + s.scope.UCloudCluster.ClusterName)
		}
		// newULB, err := s.ulbClient.CreateULB(req)
		// newULB, err := s.createULB(req)
		var newULB ulb.CreateULBResponse
		err = s.doRequest(req, &newULB)
		if err != nil {
			return errors.Errorf("create ulb failed: %s", err.Error())
		}

		// create eip
		eip, err := s.createEIP(ulbSpec.EIP)
		if err != nil {
			return err
		}

		// bind eip
		err = s.bindEIP(eip.EIPId, newULB.ULBId, "ulb")
		if err != nil {
			return err
		}

		// create vserver
		reqVserver := s.ulbClient.NewCreateVServerRequest()
		reqVserver.Region = ucloud.String(s.scope.Region())
		reqVserver.ProjectId = ucloud.String(s.scope.ProjectId())
		reqVserver.ULBId = ucloud.String(newULB.ULBId)
		reqVserver.Protocol = ucloud.String("TCP")
		reqVserver.MonitorType = ucloud.String("Port")
		reqVserver.FrontendPort = ucloud.Int(6443)
		reqVserver.ListenType = ucloud.String("RequestProxy")
		if ulbSpec.VServerName != "" {
			reqVserver.VServerName = ucloud.String(ulbSpec.VServerName)
		} else {
			reqVserver.VServerName = ucloud.String("k8s-api-server")
		}
		newVserver, err := s.ulbClient.CreateVServer(reqVserver)
		if err != nil {
			return errors.Errorf("create vserver failed: %s", err.Error())
		}

		finalULB = &ulb.ULBSet{
			Bandwidth: eip.Bandwidth,
			// IPSet:         nil,
			Name:    ucloud.StringValue(req.ULBName),
			ULBId:   newULB.ULBId,
			ULBType: "OuterMode",
			VPCId:   vpcId,
			// VServerSet:    nil,
		}
		finalULB.IPSet = append(finalULB.IPSet, ulb.ULBIPSet{
			Bandwidth: eip.Bandwidth,
			EIP:       eip.EIPAddr,
			EIPId:     eip.EIPId,
		})
		finalULB.VServerSet = append(finalULB.VServerSet, ulb.ULBVServerSet{
			// BackendSet:      nil,
			FrontendPort: 6443,
			ListenType:   "PacketsTransmit",
			Method:       "Roundrobin",
			MonitorType:  "Port",
			Protocol:     "TCP",
			VServerId:    newVserver.VServerId,
			VServerName:  ucloud.StringValue(reqVserver.VServerName),
		})

	}

	s.scope.Info("reconcile ulb success", "status", finalULB)

	s.scope.UCloudCluster.Status.Network.ULB.LoadBalancerId = finalULB.ULBId
	s.scope.UCloudCluster.Status.Network.ULB.LoadBalancerName = finalULB.Name
	s.scope.UCloudCluster.Status.Network.ULB.VpcId = finalULB.VPCId
	s.scope.UCloudCluster.Status.Network.ULB.VServerId = finalULB.VServerSet[0].VServerId
	s.scope.UCloudCluster.Status.Network.ULB.EIP.EIPId = finalULB.IPSet[0].EIPId
	s.scope.UCloudCluster.Status.Network.ULB.EIP.EIPAddr = finalULB.IPSet[0].EIP
	s.scope.UCloudCluster.Status.Network.ULB.EIP.Bandwidth = finalULB.IPSet[0].Bandwidth
	return nil
}

func (s *Service) DeleteULB() error {

	s.scope.Info("delete ulb")

	id := s.scope.UCloudCluster.Status.Network.ULB.LoadBalancerId
	if len(id) == 0 {
		return nil
	}
	if s.scope.UCloudCluster.Spec.Network.ULB.LoadBalancerId == id {
		s.scope.Info("ulb is not created by cluster-api-provider-ucloud, will not be deleted", "ulbid", id)
		return nil
	}
	if err := s.deleteULB(id); err != nil {
		return err
	}
	s.scope.Info("delete ulb success", "subnetid", id)
	return nil
}

func (s *Service) deleteULB(ulbId string) error {
	// delete ulb
	delReq := s.ulbClient.NewDeleteULBRequest()
	delReq.Region = ucloud.String(s.scope.Region())
	delReq.ProjectId = ucloud.String(s.scope.ProjectId())
	delReq.ULBId = ucloud.String(ulbId)
	delReq.ReleaseEip = ucloud.Bool(true)
	res, err := s.ulbClient.DeleteULB(delReq)
	if err != nil && res.GetRetCode() != 63059 {
		return errors.Errorf("delete ulb %s failed: %s", ulbId, err.Error())
	}
	return nil
}

func (s *Service) AddRealServer(hostId string) error {
	req := s.ulbClient.NewDescribeVServerRequest()
	req.Region = ucloud.String(s.scope.Region())
	req.ProjectId = ucloud.String(s.scope.ProjectId())
	req.ULBId = ucloud.String(s.scope.UCloudCluster.Status.Network.ULB.LoadBalancerId)
	req.VServerId = ucloud.String(s.scope.UCloudCluster.Status.Network.ULB.VServerId)
	res, err := s.ulbClient.DescribeVServer(req)
	if err != nil {
		return errors.Wrapf(err, "describe vserver failed")
	}
	if len(res.DataSet) == 0 {
		return errors.Errorf("vserver %s not exist", req.VServerId)
	}
	for _, backend := range res.DataSet[0].BackendSet {
		if backend.ResourceId == hostId {
			return nil
		}
	}
	reqAddRS := s.ulbClient.NewAllocateBackendRequest()
	reqAddRS.Region = req.Region
	reqAddRS.ProjectId = req.ProjectId
	reqAddRS.ULBId = req.ULBId
	reqAddRS.VServerId = req.VServerId
	reqAddRS.ResourceType = ucloud.String("UHost")
	reqAddRS.ResourceId = ucloud.String(hostId)
	reqAddRS.Port = ucloud.Int(6443)
	_, err = s.ulbClient.AllocateBackend(reqAddRS)
	if err != nil {
		return errors.Wrapf(err, "add resource %s to vserver %s failed", hostId, ucloud.StringValue(req.VServerId))
	}
	return nil
}

func (s *Service) DelRealServer(hostId string) error {
	req := s.ulbClient.NewDescribeVServerRequest()
	req.Region = ucloud.String(s.scope.Region())
	req.ProjectId = ucloud.String(s.scope.ProjectId())
	req.ULBId = ucloud.String(s.scope.UCloudCluster.Status.Network.ULB.LoadBalancerId)
	req.VServerId = ucloud.String(s.scope.UCloudCluster.Status.Network.ULB.VServerId)
	res, err := s.ulbClient.DescribeVServer(req)
	if err != nil {
		return errors.Wrapf(err, "describe vserver failed")
	}
	if len(res.DataSet) == 0 {
		return errors.Errorf("vserver %s not exist", req.VServerId)
	}
	backendId := ""
	for _, backend := range res.DataSet[0].BackendSet {
		if backend.ResourceId == hostId {
			backendId = backend.BackendId
			break
		}
	}
	if backendId != "" {
		req := s.ulbClient.NewReleaseBackendRequest()
		req.Region = ucloud.String(s.scope.Region())
		req.ProjectId = ucloud.String(s.scope.ProjectId())
		req.ULBId = ucloud.String(s.scope.UCloudCluster.Status.Network.ULB.LoadBalancerId)
		req.BackendId = ucloud.String(backendId)
		_, err := s.ulbClient.ReleaseBackend(req)
		if err != nil {
			return errors.Wrapf(err, "del realserver %s failed", hostId)
		}
	}
	return nil
}

type CreateULBRequestPlus struct {
	request.CommonBase

	// ULB 所属的业务组ID，如果不传则使用默认的业务组
	BusinessId *string `required:"false"`

	// 付费方式, 枚举值为: Year, 按年付费; Month, 按月付费; Dynamic, 按时付费
	ChargeType *string `required:"false"`

	// 防火墙ID，如果不传，则默认不绑定防火墙
	FirewallId *string `required:"false"`

	// 创建的ULB是否为内网模式
	InnerMode *string `required:"false"`

	// 创建的ULB是否为外网模式，默认即为外网模式
	OuterMode *string `required:"false"`

	// 备注
	Remark *string `required:"false"`

	// 内网ULB 所属的子网ID，如果不传则使用默认的子网
	SubnetId *string `required:"false"`

	// 业务组
	Tag *string `required:"false"`

	// 负载均衡的名字，默认值为“ULB”
	ULBName *string `required:"false"`

	// ULB所在的VPC的ID, 如果不传则使用默认的VPC
	VPCId *string `required:"false"`

	ListenType *string `required:"false"`
}
