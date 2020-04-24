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
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/ucloud/ucloud-sdk-go/ucloud"
	"github.com/ucloud/ucloud-sdk-go/ucloud/request"
	"github.com/ucloud/ucloud-sdk-go/ucloud/response"
	"sigs.k8s.io/cluster-api-provider-ucloud/cloud/common"
)

func (s *Service) ReconcileUGroup() error {
	if len(s.scope.UCloudCluster.Status.Group.GroupId) > 0 {
		return nil
	}
	s.scope.Info("reconcile UGroup")
	byteArr := []byte(fmt.Sprintf("%s-%s-%s-%s-%s-%s", s.scope.ProjectId(), s.scope.Region(), s.scope.Namespace(), s.scope.UCloudCluster.Name, s.scope.UCloudCluster.Spec.Version, s.scope.Cluster.GetCreationTimestamp().String()))
	groupName := "capu-" + uuid.NewSHA1(uuid.MustParse(common.ClusterApiUUIDNamespace), byteArr).String()
	// check if group exist
	var finalGroup *BusinessGroupInfo
	req := &ListBusinessGroupRequest{}
	req.Region = ucloud.String(s.scope.Region())
	req.ProjectId = ucloud.String(s.scope.ProjectId())
	req.SetAction("ListBusinessGroup")
	req.SetRequestTime(time.Now())
	var res ListBusinessGroupResponse
	err := s.doRequest(req, &res)
	if err != nil {
		return errors.Wrap(err, "list business group failed")
	}
	groupExist := false
	for _, groupInfo := range res.Infos {
		if groupInfo.BusinessName == groupName {
			finalGroup = &groupInfo
			groupExist = true
			break
		}
	}
	if !groupExist {
		req := &CreateBusinessGroupRequest{}
		req.SetAction("CreateBusinessGroup")
		req.SetRequestTime(time.Now())
		req.Region = ucloud.String(s.scope.Region())
		req.ProjectId = ucloud.String(s.scope.ProjectId())
		req.BusinessName = ucloud.String(groupName)
		var res CreateBusinessGroupResponse
		err := s.doRequest(req, &res)
		if err != nil {
			return errors.Wrap(err, "create business group failed")
		}
		finalGroup = &BusinessGroupInfo{
			BusinessId:   res.BusinessId,
			BusinessName: res.BusinessName,
		}
	}

	s.scope.Info("reconcile group success", "status", finalGroup)

	s.scope.UCloudCluster.Status.Group.GroupId = finalGroup.BusinessId
	s.scope.UCloudCluster.Status.Group.GroupName = finalGroup.BusinessName
	return nil
}

func (s *Service) DeleteGroup() error {
	s.scope.Info("delete group")
	id := s.scope.UCloudCluster.Status.Group.GroupId
	if len(id) == 0 {
		return nil
	}

	delReq := &DeleteBusinessGroupRequest{}
	delReq.SetAction("DeleteBusinessGroup")
	delReq.SetRequestTime(time.Now())
	delReq.Region = ucloud.String(s.scope.Region())
	delReq.ProjectId = ucloud.String(s.scope.ProjectId())
	delReq.BusinessId = ucloud.String(id)
	var res DeleteBusinessGroupResponse
	err := s.doRequest(delReq, &res)
	if err != nil {
		return errors.Wrap(err, "delete business group failed")
	}
	s.scope.Info("delete group success", "groupid", id)
	return nil
}

func (s *Service) CleanResourceInGroup() error {
	s.scope.Info("clean resource in group")
	id := s.scope.UCloudCluster.Status.Group.GroupId
	if len(id) == 0 {
		return nil
	}
	// clean ulbs in group
	searchReq := &SearchBusinessGroupResourceRequest{}
	searchReq.SetAction("SearchBusinessGroupResource")
	searchReq.SetRequestTime(time.Now())
	searchReq.ProjectId = ucloud.String(s.scope.ProjectId())
	searchReq.BusinessId = ucloud.String(id)
	var searchRes SearchBusinessGroupResourceResponse
	err := s.doRequest(searchReq, &searchRes)
	if err != nil {
		return errors.Wrap(err, "search resource in business group failed")
	}
	if searchRes.TotalCount > 10 {
		searchReq.Limit = ucloud.String(strconv.Itoa(searchRes.TotalCount))
		err := s.doRequest(searchReq, &searchRes)
		if err != nil {
			return errors.Wrap(err, "search resource in business group failed")
		}
	}
	for _, resource := range searchRes.Infos {
		switch strings.ToLower(resource.ResourceTypeName) {
		case "ulb":
			if err := s.deleteULB(resource.Id); err != nil {
				return errors.Wrap(err, "clean ulb in business group failed")
			}
		case "uhost":
			if err := s.terminateUHost(resource.Id, resource.ZoneId); err != nil {
				return errors.Wrap(err, "clean uhost in business group failed")
			}
		}
	}
	s.scope.Info("clean resource in group success", "groupid", id)
	return nil
}

type ListBusinessGroupRequest struct {
	request.CommonBase
}

// ListBusinessGroupResponse
type ListBusinessGroupResponse struct {
	response.CommonBase
	Infos []BusinessGroupInfo
}

type BusinessGroupInfo struct {
	BusinessId   string
	BusinessName string
}

// DeleteBusinessGroupRequest
type DeleteBusinessGroupRequest struct {
	request.CommonBase
	BusinessId *string `required:"true"`
}

// DeleteBusinessGroupResponse
type DeleteBusinessGroupResponse struct {
	response.CommonBase
}

// CreateBusinessGroupRequest
type CreateBusinessGroupRequest struct {
	request.CommonBase
	BusinessName *string `required:"true"`
}

// CreateBusinessGroupResponse
type CreateBusinessGroupResponse struct {
	response.CommonBase
	BusinessId   string
	BusinessName string
}

// SearchBusinessGroupResourceRequest
type SearchBusinessGroupResourceRequest struct {
	request.CommonBase
	BusinessId *string `required:"true"`
	Limit      *string
	Offset     *string
}

// SearchBusinessGroupResourceResponse
type SearchBusinessGroupResourceResponse struct {
	response.CommonBase
	TotalCount int
	Infos      []ResourceInfo
}

type ResourceInfo struct {
	Id               string
	ResourceId       string
	RegionId         string
	ZoneId           string
	ResourceType     int
	ResourceTypeName string
}
