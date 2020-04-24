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

package common

import (
	"fmt"
)

const (
	// DefaultUserName is the default username for created vm
	DefaultUserName = "capi"
	// DefaultVnetCIDR is the default Vnet CIDR
	DefaultVnetCIDR = "10.0.0.0/8"
	// DefaultControlPlaneSubnetCIDR is the default Control Plane Subnet CIDR
	DefaultControlPlaneSubnetCIDR = "10.0.0.0/16"
	// DefaultNodeSubnetCIDR is the default Node Subnet CIDR
	DefaultNodeSubnetCIDR = "10.1.0.0/16"
	// DefaultInternalLBIPAddress is the default internal load balancer ip address
	DefaultInternalLBIPAddress = "10.0.0.100"
	// UserAgent used for communicating with ucloud
	UserAgent = "cluster-api-ucloud-services"
	// DefaultNatGatewayEipBandwidth 10Mb
	DefaultNatGatewayEIPBandwidth = 10
	// DefaultUHostCPU 4
	DefaultUHostCPU = 4
	// DefaultUHostMemory 8192 MB
	DefaultUHostMemory = 8192
	//DefaultRootDiskSize 40 GB
	DefaultUHostRootDiskSize = 40
	// DefaultDataDiskSize 40 GB
	DefaultUHostDataDiskSize = 40
	// ClusterApiUUIDNamespace
	ClusterApiUUIDNamespace = "e364031d-ad93-4744-b411-85bb870a8623"
)

var RegionZoneMap = map[string][]string{
	"cn-bj1":       {"cn-bj1-01"},
	"cn-bj2":       {"cn-bj2-02", "cn-bj2-03", "cn-bj2-04", "cn-bj2-05"},
	"cn-sh":        {"cn-sh-02", "cn-sh-03"},
	"cn-sh2":       {"cn-sh2-02", "cn-sh2-03"},
	"cn-gd":        {"cn-gd-02"},
	"cn-gd2":       {"cn-gd2-01"},
	"hk":           {"hk-01", "hk-02"},
	"tw-tp":        {"tw-tp-01"},
	"tw-tp2":       {"tw-tp2-01"},
	"tw-kh":        {"tw-kh-01"},
	"jpn-tky":      {"jpn-tky-01"},
	"kr-seoul":     {"kr-seoul-01"},
	"th-bkk":       {"th-bkk-01"},
	"sg":           {"sg-01"},
	"idn-jakarta":  {"idn-jakarta-01"},
	"vn-sng":       {"vn-sng-01"},
	"us-ca":        {"us-ca-01"},
	"us-ws":        {"us-ws-01"},
	"rus-mosc":     {"rus-mosc-01"},
	"ge-fra":       {"ge-fra-01"},
	"uk-london":    {"uk-london-01"},
	"ind-mumbai":   {"ind-mumbai-01"},
	"uae-dubai":    {"uae-dubai-01"},
	"bra-saopaulo": {"bra-saopaulo-01"},
	"afr-nigeria":  {"afr-nigeria-01"},
}

var RegionImageMap = map[string](map[string]string){
	"cn-bj1":       nil,
	"cn-bj2":       {"cn-bj2-02": "uimage-kdwczn", "cn-bj2-03": "uimage-aodbek", "cn-bj2-04": "uimage-igmic5", "cn-bj2-05": "uimage-edkznm"},
	"cn-sh":        nil,
	"cn-sh2":       nil,
	"cn-gd":        nil,
	"cn-gd2":       nil,
	"hk":           nil,
	"tw-tp":        nil,
	"tw-tp2":       nil,
	"tw-kh":        nil,
	"jpn-tky":      nil,
	"kr-seoul":     nil,
	"th-bkk":       nil,
	"sg":           nil,
	"idn-jakarta":  nil,
	"vn-sng":       nil,
	"us-ca":        nil,
	"us-ws":        nil,
	"rus-mosc":     nil,
	"ge-fra":       nil,
	"uk-london":    nil,
	"ind-mumbai":   nil,
	"uae-dubai":    nil,
	"bra-saopaulo": nil,
	"afr-nigeria":  nil,
}

var RegionEIPOperator = map[string]string{
	"cn-bj1":       "Bgp",
	"cn-bj2":       "Bgp",
	"cn-sh":        "Bgp",
	"cn-sh2":       "Bgp",
	"cn-gd":        "Bgp",
	"cn-gd2":       "Bgp",
	"hk":           "International",
	"tw-tp":        "International",
	"tw-tp2":       "International",
	"tw-kh":        "International",
	"jpn-tky":      "International",
	"kr-seoul":     "International",
	"th-bkk":       "International",
	"sg":           "International",
	"idn-jakarta":  "International",
	"vn-sng":       "International",
	"us-ca":        "International",
	"us-ws":        "International",
	"rus-mosc":     "International",
	"ge-fra":       "International",
	"uk-london":    "International",
	"ind-mumbai":   "International",
	"uae-dubai":    "International",
	"bra-saopaulo": "International",
	"afr-nigeria":  "International",
}

// GenerateVnetName generates a virtual network name, based on the cluster name.
func GenerateVnetName(clusterName string) string {
	return fmt.Sprintf("%s-%s", clusterName, "vnet")
}

// GenerateControlPlaneSecurityGroupName generates a control plane security group name, based on the cluster name.
func GenerateControlPlaneSecurityGroupName(clusterName string) string {
	return fmt.Sprintf("%s-%s", clusterName, "controlplane-nsg")
}

// GenerateNodeSecurityGroupName generates a node security group name, based on the cluster name.
func GenerateNodeSecurityGroupName(clusterName string) string {
	return fmt.Sprintf("%s-%s", clusterName, "node-nsg")
}

// GenerateNodeRouteTableName generates a node route table name, based on the cluster name.
func GenerateNodeRouteTableName(clusterName string) string {
	return fmt.Sprintf("%s-%s", clusterName, "node-routetable")
}

// GenerateControlPlaneSubnetName generates a node subnet name, based on the cluster name.
func GenerateControlPlaneSubnetName(clusterName string) string {
	return fmt.Sprintf("%s-%s", clusterName, "controlplane-subnet")
}

// GenerateNodeSubnetName generates a node subnet name, based on the cluster name.
func GenerateNodeSubnetName(clusterName string) string {
	return fmt.Sprintf("%s-%s", clusterName, "node-subnet")
}

// GenerateInternalLBName generates a internal load balancer name, based on the cluster name.
func GenerateInternalLBName(clusterName string) string {
	return fmt.Sprintf("%s-%s", clusterName, "internal-lb")
}

// GeneratePublicLBName generates a public load balancer name, based on the cluster name.
func GeneratePublicLBName(clusterName string) string {
	return fmt.Sprintf("%s-%s", clusterName, "public-lb")
}

// GeneratePublicIPName generates a public IP name, based on the cluster name and a hash.
func GeneratePublicIPName(clusterName, hash string) string {
	return fmt.Sprintf("%s-%s", clusterName, hash)
}

// GenerateNICName generates the name of a network interface based on the name of a VM.
func GenerateNICName(machineName string) string {
	return fmt.Sprintf("%s-nic", machineName)
}

// GenerateOSDiskName generates the name of an OS disk based on the name of a VM.
func GenerateOSDiskName(machineName string) string {
	return fmt.Sprintf("%s_OSDisk", machineName)
}
