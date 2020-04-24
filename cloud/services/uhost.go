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
	"encoding/base64"
	"encoding/json"
	"math/rand"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/ucloud/ucloud-sdk-go/services/uhost"
	"github.com/ucloud/ucloud-sdk-go/ucloud"
	"sigs.k8s.io/cluster-api/util/record"

	infrav1 "sigs.k8s.io/cluster-api-provider-ucloud/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-ucloud/cloud/common"
	"sigs.k8s.io/cluster-api-provider-ucloud/cloud/scope"
)

// InstanceIfExists returns the existing instance or nothing if it doesn't exist.
func (s *Service) InstanceIfExists(scope *scope.MachineScope) (*uhost.UHostInstanceSet, error) {
	id := scope.GetInstanceID()
	if id == nil {
		return nil, nil
	}
	s.scope.Info("looking for instance by id", "name", scope.Name(), "id", *id)
	req := s.uhostClient.NewDescribeUHostInstanceRequest()
	req.Region = ucloud.String(s.scope.Region())
	req.ProjectId = ucloud.String(s.scope.ProjectId())
	req.Zone = ucloud.String(scope.GetZone())
	req.UHostIds = append(req.UHostIds, *id)
	req.Tag = ucloud.String(s.scope.GroupName())
	hosts, err := s.uhostClient.DescribeUHostInstance(req)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to describe instance: %s", *id)
	}
	if len(hosts.UHostSet) == 0 {
		return nil, nil
	}
	return &(hosts.UHostSet[0]), nil
}

// CreateInstance runs a uhost instance.
func (s *Service) CreateInstance(scope *scope.MachineScope) (*uhost.UHostInstanceSet, error) {
	s.scope.Info("Creating an instance")
	bootstrapData, err := scope.GetBootstrapData()
	if err != nil {
		record.Warnf(scope.Machine, "FailedCreate", "Failed to create instance")
		return nil, errors.Wrap(err, "failed to retrieve bootstrap data")
	}

	imageId := s.getImageId(scope)
	if imageId == "" {
		record.Warnf(scope.Machine, "FailedCreate", "Failed to create instance")
		return nil, errors.Errorf("can not get image id for region %s", s.scope.Region())
	}
	s.scope.Info("use image", "imageid", imageId)
	req := s.uhostClient.NewCreateUHostInstanceRequest()
	req.Region = ucloud.String(s.scope.Region())
	req.ProjectId = ucloud.String(s.scope.ProjectId())
	req.Zone = ucloud.String(scope.Zone())
	req.Tag = ucloud.String(s.scope.GroupName())
	if ucloud.StringValue(req.Zone) == "" {
		zones, ok := common.RegionZoneMap[s.scope.Region()]
		if !ok {
			record.Warnf(scope.Machine, "FailedCreate", "Failed to create instance")
			return nil, errors.Errorf("region %s not support", s.scope.Region())
		}
		req.Zone = ucloud.String(zones[rand.Intn(len(zones))])
	}
	if scope.UCloudMachine.Name != "" {
		req.Name = ucloud.String(scope.UCloudCluster.Namespace + "-" + scope.UCloudMachine.Name)
	} else {
		req.Name = ucloud.String(scope.UCloudCluster.Namespace + "-" + scope.UCloudCluster.ClusterName + "-" + scope.Role())
	}
	req.ChargeType = ucloud.String("Month")
	req.Quantity = ucloud.Int(1)
	req.VPCId = ucloud.String(s.scope.UCloudCluster.Status.Network.VPC.VpcId)
	req.SubnetId = ucloud.String(s.scope.UCloudCluster.Status.Network.Subnet.SubnetId)
	req.ImageId = ucloud.String(imageId)
	if scope.UCloudMachine.Spec.CPU != 0 {
		req.CPU = ucloud.Int(scope.UCloudMachine.Spec.CPU)
	} else {
		req.CPU = ucloud.Int(common.DefaultUHostCPU)
	}
	if scope.UCloudMachine.Spec.Memory != 0 {
		req.Memory = ucloud.Int(scope.UCloudMachine.Spec.Memory)
	} else {
		req.Memory = ucloud.Int(common.DefaultUHostMemory)
	}
	rootDiskSize := scope.UCloudMachine.Spec.RootDiskSize
	if rootDiskSize == 0 {
		rootDiskSize = common.DefaultUHostRootDiskSize
	}
	req.Disks = append(req.Disks, uhost.UHostDisk{
		Size:       ucloud.Int(rootDiskSize),
		Type:       ucloud.String("CLOUD_SSD"),
		IsBoot:     ucloud.String("true"),
		BackupType: ucloud.String("NONE"),
	})
	dataDiskSize := scope.UCloudMachine.Spec.DataDiskSize
	if dataDiskSize == 0 {
		dataDiskSize = common.DefaultUHostDataDiskSize
	}
	req.Disks = append(req.Disks, uhost.UHostDisk{
		Size:       ucloud.Int(dataDiskSize),
		Type:       ucloud.String("CLOUD_SSD"),
		IsBoot:     ucloud.String("false"),
		BackupType: ucloud.String("NONE"),
	})
	req.MachineType = ucloud.String("N")
	req.MinimalCpuPlatform = ucloud.String("Intel/Auto")

	req.LoginMode = ucloud.String("Password")
	passwd := scope.UCloudMachine.Spec.SSHPassword
	if passwd == "" {
		record.Warnf(scope.Machine, "FailedCreate", "Failed to create instance")
		return nil, errors.Errorf("password is not set")
	}
	pass, err := base64.StdEncoding.DecodeString(passwd)
	if err != nil {
		return nil, errors.Wrap(err, "sshPassword is not a valid base64 string")
	}
	req.Password = ucloud.String(string(pass))
	bootstrapData = strings.ReplaceAll(bootstrapData, `## template: jinja`, "")
	credentailData, err := json.Marshal(*s.scope.Credential)
	if err != nil {
		return nil, errors.Wrap(err, "marshal credentail failed")
	}
	bootstrapData = strings.ReplaceAll(bootstrapData, "UCLOUD_CREDENTIAL", base64.StdEncoding.EncodeToString(credentailData))
	bootstrapData = strings.ReplaceAll(bootstrapData, "KUBERNETES_VERSION", *scope.Machine.Spec.Version)
	req.UserData = ucloud.String(base64.StdEncoding.EncodeToString([]byte(bootstrapData)))

	newUHost, err := s.uhostClient.CreateUHostInstance(req)
	if err != nil {
		record.Warnf(scope.Machine, "FailedCreate", "Failed to create instance")
		return nil, errors.Wrap(err, "create uhost failed")
	}
	s.scope.Info("instance created successed, begin waiting for instance running", "uhostid", newUHost.UHostIds[0])

	reqDescribe := s.uhostClient.NewDescribeUHostInstanceRequest()
	reqDescribe.Region = req.Region
	reqDescribe.ProjectId = req.ProjectId
	reqDescribe.Zone = req.Zone
	reqDescribe.UHostIds = newUHost.UHostIds
	reqDescribe.Tag = ucloud.String(s.scope.GroupName())
	var finalHost uhost.UHostInstanceSet
	timer := time.NewTimer(5 * time.Minute)
	for {
		s.scope.Info("waiting for uhost ready", "uhostid", newUHost.UHostIds[0])
		hosts, err := s.uhostClient.DescribeUHostInstance(reqDescribe)
		if err != nil {
			record.Warnf(scope.Machine, "FailedCreate", "Failed to create instance")
			return nil, errors.Wrap(err, "describe uhost failed")
		}
		finalHost = hosts.UHostSet[0]
		if finalHost.State == string(uhost.StateRunning) {
			timer.Stop()
			break
		}
		select {
		case <-timer.C:
			record.Warnf(scope.Machine, "FailedCreate", "Failed to create instance")
			return nil, errors.Errorf("waiting for uhost %s ready timeout", finalHost.UHostId)
		default:
			time.Sleep(30 * time.Second)
		}
	}

	s.scope.Info("create uhost successed", "uhostid", finalHost.UHostId)
	record.Eventf(scope.Machine, "SuccessfulCreate", "Created new %s instance with name %q", scope.Role(), finalHost.Name)
	return &finalHost, nil
}

func (s *Service) TerminateInstanceAndWait(scope *scope.MachineScope) error {
	id := scope.GetInstanceID()
	if id == nil || *id == "" {
		return nil
	}
	s.scope.Info("start terminate uhost", "uhostid", *id)

	if err := s.terminateUHost(*id, scope.Zone()); err != nil {
		return err
	}

	s.scope.Info("terminate uhost successed", "uhostid", *id)

	return nil
}

// getImageId computes the UHost image id to use as the boot disk
func (s *Service) getImageId(scope *scope.MachineScope) string {
	if scope.UCloudMachine.Spec.ImageId != nil {
		return *scope.UCloudMachine.Spec.ImageId
	} else {
		return common.RegionImageMap[s.scope.Region()]
	}
}

// CreateBastionInstance runs a uhost instance.
func (s *Service) CreateBastionInstance() error {
	if s.scope.UCloudCluster.Spec.Bastion.SSHPassword == "" || s.scope.UCloudCluster.Status.Bastion != nil {
		return nil
	}

	bastionName := s.scope.UCloudCluster.Namespace + "-" + s.scope.UCloudCluster.Name + "-bastion"

	// check if exist already
	reqCheck := s.uhostClient.NewDescribeUHostInstanceRequest()
	reqCheck.Region = ucloud.String(s.scope.Region())
	reqCheck.ProjectId = ucloud.String(s.scope.ProjectId())
	reqCheck.SubnetId = ucloud.String(s.scope.UCloudCluster.Status.Network.Subnet.SubnetId)
	reqCheck.Tag = ucloud.String(s.scope.GroupName())
	hostSet, err := s.uhostClient.DescribeUHostInstance(reqCheck)
	if err != nil {
		return errors.Wrapf(err, "failed to describe instance")
	}
	for _, host := range hostSet.UHostSet {
		if host.Name == bastionName {
			return nil
		}
	}

	imageId := common.RegionImageMap[s.scope.Region()]
	if imageId == "" {
		record.Warnf(s.scope.UCloudCluster, "FailedCreate", "Failed to create instance")
		return errors.Errorf("can not get image id for region %s", s.scope.Region())
	}
	s.scope.Info("use image", "imageid", imageId)
	req := s.uhostClient.NewCreateUHostInstanceRequest()
	req.Region = ucloud.String(s.scope.Region())
	req.ProjectId = ucloud.String(s.scope.ProjectId())
	req.Zone = ucloud.String(s.scope.UCloudCluster.Spec.Bastion.Zone)
	req.Tag = ucloud.String(s.scope.GroupName())
	if ucloud.StringValue(req.Zone) == "" {
		zones, ok := common.RegionZoneMap[s.scope.Region()]
		if !ok {
			record.Warnf(s.scope.UCloudCluster, "FailedCreate", "Failed to create instance")
			return errors.Errorf("region %s not support", s.scope.Region())
		}
		req.Zone = ucloud.String(zones[rand.Intn(len(zones))])
	}
	req.Name = ucloud.String(bastionName)
	req.ChargeType = ucloud.String("Month")
	req.Quantity = ucloud.Int(1)
	req.VPCId = ucloud.String(s.scope.UCloudCluster.Status.Network.VPC.VpcId)
	req.SubnetId = ucloud.String(s.scope.UCloudCluster.Status.Network.Subnet.SubnetId)
	req.ImageId = ucloud.String(imageId)
	req.CPU = ucloud.Int(2)
	req.Memory = ucloud.Int(4096)
	rootDiskSize := common.DefaultUHostRootDiskSize
	req.Disks = append(req.Disks, uhost.UHostDisk{
		Size:       ucloud.Int(rootDiskSize),
		Type:       ucloud.String("CLOUD_SSD"),
		IsBoot:     ucloud.String("true"),
		BackupType: ucloud.String("NONE"),
	})
	req.MachineType = ucloud.String("N")
	req.MinimalCpuPlatform = ucloud.String("Intel/Auto")
	req.NetworkInterface = append(req.NetworkInterface, uhost.CreateUHostInstanceParamNetworkInterface{
		EIP: &uhost.CreateUHostInstanceParamNetworkInterfaceEIP{
			Bandwidth:    ucloud.Int(1),
			OperatorName: ucloud.String(common.RegionEIPOperator[s.scope.Region()]),
			PayMode:      ucloud.String("Bandwidth"),
		},
	})

	req.LoginMode = ucloud.String("Password")
	passwd := s.scope.UCloudCluster.Spec.Bastion.SSHPassword
	if passwd == "" {
		record.Warnf(s.scope.UCloudCluster, "FailedCreate", "Failed to create bastion")
		return errors.Errorf("password is not set")
	}
	pass, err := base64.StdEncoding.DecodeString(passwd)
	if err != nil {
		return errors.Wrap(err, "sshPassword is not a valid base64 string")
	}
	req.Password = ucloud.String(string(pass))

	newUHost, err := s.uhostClient.CreateUHostInstance(req)
	if err != nil {
		record.Warnf(s.scope.UCloudCluster, "FailedCreate", "Failed to create bastion")
		return errors.Wrap(err, "create uhost failed")
	}

	reqDescribe := s.uhostClient.NewDescribeUHostInstanceRequest()
	reqDescribe.Region = req.Region
	reqDescribe.ProjectId = req.ProjectId
	reqDescribe.Zone = req.Zone
	reqDescribe.UHostIds = newUHost.UHostIds
	reqDescribe.Tag = ucloud.String(s.scope.GroupName())
	var finalHost uhost.UHostInstanceSet
	hosts, err := s.uhostClient.DescribeUHostInstance(reqDescribe)
	if err != nil {
		record.Warnf(s.scope.UCloudCluster, "FailedCreate", "Failed to create bastion")
		return errors.Wrap(err, "describe uhost failed")
	}
	finalHost = hosts.UHostSet[0]

	s.scope.Info("create uhost successed", "uhostid", finalHost.UHostId)
	bastionInfo := infrav1.Instance{
		InstanceId:   finalHost.UHostId,
		PrivateIP:    s.getPrivateIP(&finalHost),
		PublicIP:     s.getPublicIP(&finalHost),
		InstanceType: "uhost",
		Zone:         finalHost.Zone,
		Name:         finalHost.Name,
	}
	s.scope.UCloudCluster.Status.Bastion = &bastionInfo
	record.Eventf(s.scope.UCloudCluster, "SuccessfulCreate", "Created bastion with name %q", finalHost.Name)
	return nil
}

func (s *Service) TerminateBastion() error {
	if s.scope.UCloudCluster.Status.Bastion == nil || s.scope.UCloudCluster.Status.Bastion.InstanceId == "" {
		return nil
	}
	id := s.scope.UCloudCluster.Status.Bastion.InstanceId
	zone := s.scope.UCloudCluster.Status.Bastion.Zone
	s.scope.Info("start terminate instance", "uhostid", id)

	if err := s.terminateUHost(id, zone); err != nil {
		return err
	}

	s.scope.UCloudCluster.Status.Bastion = nil
	s.scope.Info("terminate uhost successed", "uhostid", id)

	return nil
}

func (s *Service) getPrivateIP(uhost *uhost.UHostInstanceSet) string {
	for _, ipInfo := range uhost.IPSet {
		if ipInfo.Default == "true" && ipInfo.Type == "Private" {
			return ipInfo.IP
		}
	}
	return ""
}

func (s *Service) getPublicIP(uhost *uhost.UHostInstanceSet) string {
	for _, ipInfo := range uhost.IPSet {
		if strings.ToLower(ipInfo.Type) == strings.ToLower(common.RegionEIPOperator[s.scope.Region()]) {
			return ipInfo.IP
		}
	}
	return ""
}

func (s *Service) terminateUHost(id, zone string) error {
	req := s.uhostClient.NewDescribeUHostInstanceRequest()
	req.Region = ucloud.String(s.scope.Region())
	req.ProjectId = ucloud.String(s.scope.ProjectId())
	req.Zone = ucloud.String(zone)
	req.UHostIds = append(req.UHostIds, id)
	req.Tag = ucloud.String(s.scope.GroupName())
	hosts, err := s.uhostClient.DescribeUHostInstance(req)
	if err != nil && hosts.GetRetCode() != 8039 {
		return errors.Wrapf(err, "failed to describe instance: %s", id)
	}
	if len(hosts.UHostSet) == 0 {
		return nil
	}

	reqPowerOff := s.uhostClient.NewPoweroffUHostInstanceRequest()
	reqPowerOff.Region = ucloud.String(s.scope.Region())
	reqPowerOff.ProjectId = ucloud.String(s.scope.ProjectId())
	reqPowerOff.Zone = req.Zone
	reqPowerOff.UHostId = ucloud.String(id)
	_, err = s.uhostClient.PoweroffUHostInstance(reqPowerOff)
	if err != nil {
		return errors.Wrapf(err, "poweroff uhost %s failed", id)
	}

	reqDescribe := s.uhostClient.NewDescribeUHostInstanceRequest()
	reqDescribe.Region = req.Region
	reqDescribe.ProjectId = req.ProjectId
	reqDescribe.Zone = req.Zone
	reqDescribe.UHostIds = append(reqDescribe.UHostIds, id)
	reqDescribe.Tag = ucloud.String(s.scope.GroupName())
	timer := time.NewTimer(5 * time.Minute)
	for {
		s.scope.Info("waiting for uhost stopped", "uhostid", id)
		hosts, err := s.uhostClient.DescribeUHostInstance(reqDescribe)
		if err != nil {
			return errors.Wrap(err, "describe uhost failed")
		}
		host := hosts.UHostSet[0]
		if host.State == string(uhost.StateStopped) {
			timer.Stop()
			break
		}
		select {
		case <-timer.C:
			return errors.Errorf("waiting for uhost %s stopped timeout", id)
		default:
			time.Sleep(10 * time.Second)
		}
	}

	reqTerminate := s.uhostClient.NewTerminateUHostInstanceRequest()
	reqTerminate.Region = req.Region
	reqTerminate.ProjectId = req.ProjectId
	reqTerminate.Zone = req.Zone
	reqTerminate.UHostId = reqPowerOff.UHostId
	reqTerminate.ReleaseEIP = ucloud.Bool(true)
	reqTerminate.ReleaseUDisk = ucloud.Bool(true)
	_, err = s.uhostClient.TerminateUHostInstance(reqTerminate)
	if err != nil {
		return errors.Wrapf(err, "terminate uhost %s failed", id)
	}
	timer.Reset(5 * time.Minute)
	for {
		s.scope.Info("waiting for uhost deleted", "uhostid", id)
		hosts, err := s.uhostClient.DescribeUHostInstance(reqDescribe)
		if err != nil {
			return errors.Wrap(err, "describe uhost failed")
		}
		if len(hosts.UHostSet) == 0 {
			timer.Stop()
			break
		}
		select {
		case <-timer.C:
			return errors.Errorf("waiting for uhost %s deleted timeout", id)
		default:
			time.Sleep(10 * time.Second)
		}
	}
	return nil
}
