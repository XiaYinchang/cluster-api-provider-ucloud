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

package v1alpha3

// UCloudMachineTemplateResource describes the data needed to create am UCloudMachine from a template
type UCloudMachineTemplateResource struct {
	// Spec is the specification of the desired behavior of the machine.
	Spec UCloudMachineSpec `json:"spec"`
}

// Filter is a filter used to identify an UCLOUD resource
type Filter struct {
	// Name of the filter. Filter names are case-sensitive.
	Name string `json:"name"`

	// Values includes one or more filter values. Filter values are case-sensitive.
	Values []string `json:"values"`
}

type Network struct {
	VPC      VPC      `json:"vpc,omitempty"`
	Subnet   Subnet   `json:"subnet,omitempty"`
	ULB      ULB      `json:"ulb,omitempty"`
	Nat      Nat      `json:"nat,omitempty"`
	Firewall Firewall `json:"firewall,omitempty"`
}

type NetworkSpec struct {
	VPC      VPCSpec      `json:"vpc,omitempty"`
	Subnet   SubnetSpec   `json:"subnet,omitempty"`
	Nat      NatSpec      `json:"nat,omitempty"`
	ULB      ULBSpec      `json:"ulb,omitempty"`
	Firewall FirewallSpec `json:"firewall,omitempty"`
}

// SubnetSpec configures an UCLOUD Subnet.

type SubnetSpec struct {
	// 使用一个已经存在的 Subnet
	SubnetId string `json:"subnetId,omitempty"`

	// 子网的名称。
	SubnetName string `json:"subnetName,omitempty"`
	// 子网的网段。子网网段要求如下：
	//   子网网段的掩码长度范围为16-29位。
	//   子网的网段必须从属于所在VPC的网段。
	//   子网的网段不能与所在VPC中路由条目的目标网段相同，但可以是目标网段的子集。
	//   如果子网的网段与所在VPC的网段相同时，VPC只能有一个子网。
	CidrBlock string `json:"cidrBlock,omitempty"`

	// 子网的描述信息。
	Description string `json:"description,omitempty"`
}

// VPCSpec 专有网络
// 使用云资源前, 必须先创建一个专有网络和子网
type VPCSpec struct {
	// 使用一个已经存在的VPC
	VpcId string `json:"vpcId,omitempty"`

	// 专有网络名称。
	VpcName string `json:"vpcName,omitempty"`
	// VPC的网段。您可以使用以下网段或其子集：
	//   10.0.0.0/8
	//   172.16.0.0/12
	//   192.168.0.0/16
	CidrBlock string `json:"cidrBlock,omitempty"`

	// VPC的描述信息。
	Description string `json:"description,omitempty"`
}

// NatSpec NAT网关相关配置, 在VPC环境下构建一个公网流量的出入口
type NatSpec struct {
	// NAT网
	NatGateway NatGatewaySpec `json:"natGateway,omitempty"`
	//
	EIP EIPSpec `json:"eip,omitempty"`
}

// NatGatewaySpec NAT网关 在VPC环境下构建一个公网流量的出入口
// 详细文档见 [CreateNatGateway]
type NatGatewaySpec struct {
	// 使用一个已经存在的NAT网关
	NatGatewayId string `json:"natGatewayId,omitempty"`

	// NAT网关的名称
	Name string `json:"name,omitempty"`

	// NAT网关的描述。
	Description string `json:"description,omitempty"`
}

// EIPSpec 弹性公网IP 配置DNAT或SNAT功能前，需要为已创建的NAT网关绑定弹性公网IP
type EIPSpec struct {
	// 使用一个已经存在的弹性公网IP
	EIPId string `json:"eipId,omitempty"`

	EIPName string `json:"eipName,omitempty"`

	// EIP的带宽峰值，单位为Mbps，默认值为5。
	Bandwidth int `json:"bandwidth,omitempty"`
}

// ULBSpec 负载均衡（Server Load Balancer）是对多台云服务器进行流量分发的负载均衡服务,
// 流量分发到apiserver
type ULBSpec struct {
	// 使用一个已经存在的负载均衡
	LoadBalancerId string `json:"loadBalancerId,omitempty"`
	// 使用一个已经存在的后端服务器组
	VServerId string `json:"vserverId,omitempty"`

	// 负载均衡实例的名称。
	LoadBalancerName string `json:"loadBalancerName,omitempty"`
	// 后端服务器组名
	VServerName string `json:"vserverName,omitempty"`

	// ULB 绑定的 EIP 信息
	EIP EIPSpec `json:"eip,omitempty"`
}

// FirewallSpec 防火墙
type FirewallSpec struct {
	// 使用一个已经存在的防火墙
	FirewallId string `json:"firewallId,omitempty"`

	// 防火墙名称。
	FirewallName string `json:"firewallName,omitempty"`
	// 防火墙入方向规则
	Rules []*FirewallRuleSpec `json:"rules,omitempty"`

	// 防火墙描述信息。
	Description string `json:"description,omitempty"`
}

// FirewallRuleSpec 防火墙入方向规则
// 详细文档见 [AuthorizeFirewall]
type FirewallRuleSpec struct {
	// 传输层协议。不区分大小写。取值范围：
	//   icmp
	//   gre
	//   tcp
	//   udp
	//   all：支持所有协议
	IpProtocol string `json:"ipProtocol,omitempty"`
	// 源端IP地址范围。支持CIDR格式和IPv4格式的IP地址范围。
	//   默认值：0.0.0.0/0。
	SourceCidrIp string `json:"sourceCidrIp,omitempty"`
	// 目的端安全组开放的传输层协议相关的端口范围。取值范围：
	//   TCP/UDP协议：取值范围为1~65535。使用斜线（/）隔开起始端口和终止端口。正确示范：1/200；错误示范：200/1。
	//   ICMP协议：-1/-1。
	//   GRE协议：-1/-1。
	//   all：-1/-1。
	PortRange string `json:"portRange,omitempty"`
	// 安全组规则的描述信息。长度为1~512个字符
	Description string `json:"description,omitempty"`
	// 访问权限。取值范围：
	//   accept：接受访问。
	//   drop：拒绝访问，不返回拒绝信息。
	//   默认值：accept。
	Policy string `json:"policy,omitempty"`
	// 源端IPv6 CIDR地址段。支持CIDR格式和IPv6格式的IP地址范围。
	//   仅支持VPC类型的IP地址。
	//   默认值：无。
	SourcePortRange string `json:"sourcePortRange,omitempty"`
	// 目的端IP地址范围。支持CIDR格式和IPv4格式的IP地址范围。
	//   默认值：0.0.0.0/0。
	DestCidrIp string `json:"destCidrIp,omitempty"`
}

///////////////////////////////

type VPC struct {
	VpcId           string `json:"vpcId,omitempty"`
	Status          string `json:"status,omitempty"`
	VpcName         string `json:"vpcName,omitempty"`
	CreationTime    string `json:"creationTime,omitempty"`
	CidrBlock       string `json:"cidrBlock,omitempty"`
	VRouterId       string `json:"vRouterId,omitempty"`
	Description     string `json:"description,omitempty"`
	IsDefault       bool   `json:"isDefault,omitempty"`
	NetworkAclNum   string `json:"networkAclNum,omitempty"`
}

type Subnet struct {
	SubnetId                string `json:"subnetId,omitempty"`
	VpcId                   string `json:"vpcId,omitempty"`
	Status                  string `json:"status,omitempty"`
	CidrBlock               string `json:"cidrBlock,omitempty"`
	ZoneId                  string `json:"zoneId,omitempty"`
	AvailableIpAddressCount int64  `json:"availableIpAddressCount,omitempty"`
	Description             string `json:"description,omitempty"`
	SubnetName              string `json:"subnetName,omitempty"`
	CreationTime            string `json:"creationTime,omitempty"`
	IsDefault               bool   `json:"isDefault,omitempty"`
	NetworkAclId            string `json:"networkAclId,omitempty"`
}

type Nat struct {
	EIP            EIP                               `json:"eip,omitempty"`
	SnatEntryId    string                            `json:"snatEntryId,omitempty"`
	Firewall       Firewall                          `json:"firewall,omitempty"`
	NatGatewayId   string                            `json:"natGatewayId,omitempty"`
	Name           string                            `json:"name,omitempty"`
	Description    string                            `json:"description,omitempty"`
	VpcId          string                            `json:"vpcId,omitempty"`
	CreationTime   string                            `json:"creationTime,omitempty"`
	Status         string                            `json:"status,omitempty"`
	SnatTableIds   SnatTableIdsInDescribeNatGateways `json:"snatTableIds,omitempty"`
}

type SnatTableIdsInDescribeNatGateways struct {
	SnatTableId []string `json:"SnatTableId" xml:"SnatTableId"`
}

type EIP struct {
	EIPId           string `json:"eipId,omitempty"`
	EIPAddr         string `json:"eipAddr,omitempty"`
	Status          string `json:"status,omitempty"`
	Bandwidth       int    `json:"bandwidth,omitempty"`
	ChargeType      string `json:"chargeType,omitempty"`
	EIPName         string `json:"eipName,omitempty"`
	Descritpion     string `json:"descritpion,omitempty"`
	Mode            string `json:"mode,omitempty"`
}

type ULB struct {
	EIP                EIP    `json:"eip,omitempty"`
	LoadBalancerId     string `json:"loadBalancerId,omitempty"`
	LoadBalancerName   string `json:"loadBalancerName,omitempty"`
	LoadBalancerStatus string `json:"loadBalancerStatus,omitempty"`
	Address            string `json:"address,omitempty"`
	VpcId              string `json:"vpcId,omitempty"`
	NetworkType        string `json:"networkType,omitempty"`
	CreateTime         string `json:"createTime,omitempty"`
	VServerId          string `json:"vserverId,omitempty"`
}

type Firewall struct {
	FirewallId      string `json:"firewallId,omitempty"`
	Description     string `json:"description,omitempty"`
	FirewallName    string `json:"firewallName,omitempty"`
	VpcId           string `json:"vpcId,omitempty"`
	CreationTime    string `json:"creationTime,omitempty"`
	FirewallType    string `json:"firewallType,omitempty"`
}

type Instance struct {
	InstanceId   string `json:"instanceId,omitempty"`
	PrivateIP    string `json:"privateIP,omitempty"`
	PublicIP     string `json:"publicIP,omitempty"`
	InstanceType string `json:"instanceType,omitempty"`
	Zone         string `json:"zone,omitempty"`
	Name         string `json:"zone,omitempty"`
}

type BastionSpec struct {
	// SSHPassword should be base64 encoded
	SSHPassword string `json:"sshPassword,omitempty"`
	// Zone if not set, will choose a zone randomly in region
	Zone string `json:"zone,omitempty"`
}

type Group struct {
	// GroupName
	GroupName string `json:"groupName,omitempty"`
	// GroupId
	GroupId string `json:"groupId,omitempty"`
}
