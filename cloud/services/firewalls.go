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
)

func (s *Service) getFirewall(firewallId string) (firewall infrav1.Firewall, err error) {

	s.scope.Info("get firewall info")
	req := s.unetClient.NewDescribeFirewallRequest()
	req.Region = ucloud.String(s.scope.Region())
	req.ProjectId = ucloud.String(s.scope.ProjectId())
	if firewallId != "" {
		req.FWId = ucloud.String(firewallId)
	}
	firewalls, err := s.unetClient.DescribeFirewall(req)
	if err != nil {
		return infrav1.Firewall{}, errors.Errorf("get firewall failed: %s", err.Error())
	}
	if len(firewalls.DataSet) == 0 {
		return infrav1.Firewall{}, errors.Errorf("can not find firewall")
	}
	finalFirewall := firewalls.DataSet[0]
	firewall.FirewallId = finalFirewall.FWId
	firewall.FirewallName = finalFirewall.Name
	firewall.FirewallType = finalFirewall.Type
	s.scope.Info("get firewall success", "firewallId", finalFirewall.FWId)
	return firewall, nil
}
