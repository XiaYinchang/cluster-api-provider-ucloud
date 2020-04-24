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

package scope

import (
	"os"

	"github.com/ucloud/ucloud-sdk-go/ucloud"
	"github.com/ucloud/ucloud-sdk-go/ucloud/auth"
)

// UCloudClients contains all the ucloud clients used by the scopes.
type UCloudClients struct {
	Config     *ucloud.Config
	Credential *auth.Credential
}

func (c *UCloudClients) loadDefaultConfig() {
	cfg := ucloud.NewConfig()
	c.Config = &cfg
}

func (c *UCloudClients) getCredentialFromEnv() {
	credential := auth.NewCredential()
	credential.PrivateKey = os.Getenv("UCLOUD_ACCESS_PRIKEY")
	credential.PublicKey = os.Getenv("UCLOUD_ACCESS_PUBKEY")
	c.Credential = &credential
}
