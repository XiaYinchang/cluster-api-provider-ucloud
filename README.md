
# Kubernetes Cluster API Provider UCLOUD

Kubernetes-native declarative infrastructure for UCLOUD.

## What is the Cluster API?

The Cluster API is a Kubernetes project to bring declarative, Kubernetes-style
APIs to cluster creation, configuration, and management. It provides optional,
additive functionality on top of core Kubernetes.

## Note

1. Only UCloud Region `cn-bj2` is supported currently.
2. cluster-api-provider-ucloud does not provide control-plane controller or bootstrap controller.
3. The kubeadm based control-plane and bootstrap controller provided by cluster-api community should be installed.

## Deploy Controller

1. Configure a management cluster following  [cluster-api doc](https://cluster-api.sigs.k8s.io/user/quick-start.html#install-andor-configure-a-kubernetes-cluster)
2. Install clusterctl following [cluster-api doc](https://cluster-api.sigs.k8s.io/user/quick-start.html#install-clusterctl).
3. init cluster-api use following command:
   ```
   clusterctl init
   ```
4. download deploy yaml from [release page](https://github.com/XiaYinchang/cluster-api-provider-ucloud/releases)
5. deploy cluster-api-provider-ucloud(replace ucloud keys with yours)
   ```
   kubectl create ns capu-system
   kubectl create secret generic manager-bootstrap-credentials -from-literal=UCLOUD_ACCESS_PUBKEY=REPLACE_WITH_YOUR_PUBLIC_KEY --from-literal=UCLOUD_ACCESS_PRIKEY=REPLACE_WITH_YOUR_PRIVATE_KEY -n capu-system
   kubectl apply -f infrastructure-components.yaml
   ```

## Deploy Demo

1. replace network info and base64 encoded ssh password for instances by yours in example/cluster.yaml
2. then apply it
   ```
   kubectl apply -f  example/cluster.yaml
   ```

## Test

1. get kubeconfig frome secret
   ```
   kubectl get secret/test-kubeconfig -o jsonpath={.data.value} | base64 --decode > /tmp/capi-ucloud.kubeconfig
   ```
2. check nodes status
   ```
   kubectl --kubeconfig=/tmp/capi-ucloud.kubeconfig get nodes -owide
   ```
3. install cni plugin
   ```
   kubectl --kubeconfig=/tmp/capi-ucloud.kubeconfig apply -f https://raw.githubusercontent.com/coreos/flannel/2140ac876ef134e0ed5af15c65e414cf26827915/Documentation/kube-flannel.yml
   ```
4. after cni plugin installed successfully, nodes will be ready and you can deploy a test app
   ```
   kubectl --kubeconfig=/tmp/capi-ucloud.kubeconfig apply -f example/test.yaml
   ```

## Bastion

If you specify `sshPassword` for `bastion` in `UCloudCluster` just like the example/cluster.yaml does. A bastion instance willed be launched. The bastion has a public ip, so you can use it to login your kubernetes nodes.

1. get public ip of bastion
   ```
   kubectl get ucloudcluster test -o jsonpath={.status.bastion.publicIP}
   ```
2. just login with the passwd you have set.

## cluster-api-uk8s-init
The tool `cluster-api-uk8s-init` used in `preKubeadmCommands` and `postKubeadmCommands` is provided by ucloud k8s team. It is neccessary for deploying cloudprovider and csi.

## IPVS

There is no where for configuring kubeproxy mode in `KubeadmControlPlane` provided by cluster-api community currently. So we provide an option in the `preKubeadmCommands` shell scripts. You can add an option `--kubeproxy-mode=ipvs` for `cluster-api-uk8s-init` like following:
```
cluster-api-uk8s-init prepare --kubeproxy-mode=ipvs --credential=UCLOUD_CREDENTIAL --node-role=master --k8s-version=KUBERNETES_VERSION --cloud-provider-version=20.04.22
```

**NOTE**: Once kubeproxy mode config is supported by cluster-api community, this approach is deprecated immediately.

## e2e-test
