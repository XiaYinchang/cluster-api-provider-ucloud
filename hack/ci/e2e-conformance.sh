#!/usr/bin/env bash

# Copyright 2019 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# hack script for running a cluster-api-provider-ucloud e2e

set -o errexit -o nounset -o pipefail

UCLOUD_PROJECT=${UCLOUD_PROJECT:-""}
UCLOUD_REGION=${UCLOUD_REGION:-"us-east4"}
CLUSTER_NAME=${CLUSTER_NAME:-"test1"}
UCLOUD_NETWORK_NAME=${UCLOUD_NETWORK_NAME:-"${CLUSTER_NAME}-mynetwork"}
KUBERNETES_MAJOR_VERSION="1"
KUBERNETES_MINOR_VERSION="17"
KUBERNETES_PATCH_VERSION="4"
KUBERNETES_VERSION="v${KUBERNETES_MAJOR_VERSION}.${KUBERNETES_MINOR_VERSION}.${KUBERNETES_PATCH_VERSION}"

TIMESTAMP=$(date +"%Y-%m-%dT%H:%M:%SZ")

ARTIFACTS="${ARTIFACTS:-${PWD}/_artifacts}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"

# dump logs from kind and all the nodes
dump-logs() {
  # log version information
  echo "=== versions ==="
  echo "kind : $(kind version)" || true
  echo "bootstrap cluster:"
  kubectl version || true
  echo "deployed cluster:"
  kubectl --kubeconfig="${PWD}"/kubeconfig version || true
  echo ""

  # dump all the info from the CAPI related CRDs
  kubectl --context=kind-clusterapi get \
    clusters,ucloudclusters,machines,ucloudmachines,kubeadmconfigs,machinedeployments,ucloudmachinetemplates,kubeadmconfigtemplates,machinesets,kubeadmcontrolplanes \
    --all-namespaces -o yaml >>"${ARTIFACTS}/logs/capg.info" || true

  # dump images info
  echo "images in docker" >>"${ARTIFACTS}/logs/images.info"
  docker images >>"${ARTIFACTS}/logs/images.info"
  echo "images from bootstrap using containerd CLI" >>"${ARTIFACTS}/logs/images.info"
  docker exec clusterapi-control-plane ctr -n k8s.io images list >>"${ARTIFACTS}/logs/images.info" || true
  echo "images in bootstrap cluster using kubectl CLI" >>"${ARTIFACTS}/logs/images.info"
  (kubectl get pods --all-namespaces -o json |
    jq --raw-output '.items[].spec.containers[].image' | sort) >>"${ARTIFACTS}/logs/images.info" || true
  echo "images in deployed cluster using kubectl CLI" >>"${ARTIFACTS}/logs/images.info"
  (kubectl --kubeconfig="${PWD}"/kubeconfig get pods --all-namespaces -o json |
    jq --raw-output '.items[].spec.containers[].image' | sort) >>"${ARTIFACTS}/logs/images.info" || true

  # dump cluster info for kind
  kubectl cluster-info dump >"${ARTIFACTS}/logs/kind-cluster.info" || true

  # dump cluster info for kind
  echo "=== ucloud compute instances list ===" >>"${ARTIFACTS}/logs/capu-cluster.info" || true
  ucloud compute instances list --project "${UCLOUD_PROJECT}" >>"${ARTIFACTS}/logs/capu-cluster.info" || true
  echo "=== cluster-info dump ===" >>"${ARTIFACTS}/logs/capu-cluster.info" || true
  kubectl --kubeconfig="${PWD}"/kubeconfig cluster-info dump >>"${ARTIFACTS}/logs/capu-cluster.info" || true

  # export all logs from kind
  kind "export" logs --name="clusterapi" "${ARTIFACTS}/logs" || true

  for node_name in $(ucloud compute instances list --filter="zone~'${UCLOUD_REGION}-.*'" --project "${UCLOUD_PROJECT}" --format='value(name)'); do
    node_zone=$(ucloud compute instances list --project "${UCLOUD_PROJECT}" --filter="name:(${node_name})" --format='value(zone)')
    echo "collecting logs from ${node_name} in zone ${node_zone}"
    dir="${ARTIFACTS}/logs/${node_name}"
    mkdir -p "${dir}"

    ucloud compute instances get-serial-port-output --project "${UCLOUD_PROJECT}" \
      --zone "${node_zone}" --port 1 "${node_name}" >"${dir}/serial-1.log" || true

    ssh-to-node "${node_name}" "${node_zone}" "sudo chmod -R a+r /var/log" || true
    ucloud compute scp --recurse --project "${UCLOUD_PROJECT}" --zone "${node_zone}" \
      "${node_name}:/var/log/cloud-init.log" "${node_name}:/var/log/cloud-init-output.log" \
      "${node_name}:/var/log/pods" "${node_name}:/var/log/containers" \
      "${dir}" || true

    ssh-to-node "${node_name}" "${node_zone}" "sudo journalctl --output=short-precise -k" >"${dir}/kern.log" || true
    ssh-to-node "${node_name}" "${node_zone}" "sudo journalctl --output=short-precise" >"${dir}/systemd.log" || true
    ssh-to-node "${node_name}" "${node_zone}" "sudo crictl version && sudo crictl info" >"${dir}/containerd.info" || true
    ssh-to-node "${node_name}" "${node_zone}" "sudo journalctl --no-pager -u kubelet.service" >"${dir}/kubelet.log" || true
    ssh-to-node "${node_name}" "${node_zone}" "sudo journalctl --no-pager -u containerd.service" >"${dir}/containerd.log" || true
  done

  ucloud logging read --order=asc \
    --format='table(timestamp,jsonPayload.resource.name,jsonPayload.event_subtype)' \
    --project "${UCLOUD_PROJECT}" \
    "timestamp >= \"${TIMESTAMP}\"" \
    >"${ARTIFACTS}/logs/activity.log" || true
}

# cleanup all resources we use
cleanup() {
  # KIND_IS_UP is true once we: kind create
  if [[ "${KIND_IS_UP:-}" = true ]]; then
    timeout 600 kubectl \
      delete cluster "${CLUSTER_NAME}" || true
    timeout 600 kubectl \
      wait --for=delete cluster/"${CLUSTER_NAME}" || true
    make kind-reset || true
  fi
  # clean up e2e.test symlink
  (cd "$(go env GOPATH)/src/k8s.io/kubernetes" && rm -f _output/bin/e2e.test) || true

  # remove our tempdir
  # NOTE: this needs to be last, or it will prevent kind delete
  if [[ -n "${TMP_DIR:-}" ]]; then
    rm -rf "${TMP_DIR}" || true
  fi
}

# our exit handler (trap)
exit-handler() {
  dump-logs
  cleanup
}

# SSH to a node by name ($1) and run a command ($2).
function ssh-to-node() {
  local node="$1"
  local zone="$2"
  local cmd="$3"

  # ensure we have an IP to connect to
  ucloud compute --project "${UCLOUD_PROJECT}" instances add-access-config --zone "${zone}" "${node}" || true

  # Loop until we can successfully ssh into the box
  for try in {1..5}; do
    if ucloud compute ssh --ssh-flag="-o LogLevel=quiet" --ssh-flag="-o ConnectTimeout=30" \
      --project "${UCLOUD_PROJECT}" --zone "${zone}" "${node}" --command "echo test > /dev/null"; then
      break
    fi
    sleep 5
  done
  # Then actually try the command.
  ucloud compute ssh --ssh-flag="-o LogLevel=quiet" --ssh-flag="-o ConnectTimeout=30" \
    --project "${UCLOUD_PROJECT}" --zone "${zone}" "${node}" --command "${cmd}"
}

# build kubernetes / node image, e2e binaries
build() {
  # possibly enable bazel build caching before building kubernetes
  if [[ "${BAZEL_REMOTE_CACHE_ENABLED:-false}" == "true" ]]; then
    create_bazel_cache_rcs.sh || true
  fi

  pushd "$(go env GOPATH)/src/k8s.io/kubernetes"

  # make sure we have e2e requirements
  bazel build //cmd/kubectl //test/e2e:e2e.test //vendor/github.com/onsi/ginkgo/ginkgo

  # ensure the e2e script will find our binaries ...
  mkdir -p "${PWD}/_output/bin/"
  cp "${PWD}/bazel-bin/test/e2e/e2e.test" "${PWD}/_output/bin/e2e.test"
  PATH="$(dirname "$(find "${PWD}/bazel-bin/" -name kubectl -type f)"):${PATH}"
  export PATH

  # attempt to release some memory after building
  sync || true
  echo 1 >/proc/sys/vm/drop_caches || true

  popd
}

# generate manifests needed for creating the UCLOUD cluster to run the tests
generate_manifests() {
  if ! command -v kustomize >/dev/null 2>&1; then
    (cd ./hack/tools/ && GO111MODULE=on go install sigs.k8s.io/kustomize/kustomize/v3)
  fi

  (UCLOUD_PROJECT=${UCLOUD_PROJECT} \
    PULL_POLICY=Never \
    make modules docker-build)

  # Enable the bits to inject a script that can pull newer versions of kubernetes
  if [[ -n ${CI_VERSION:-} || -n ${USE_CI_ARTIFACTS:-} ]]; then
    if ! grep -i -wq "patchesStrategicMerge" "templates/kustomization.yaml"; then
      echo "patchesStrategicMerge:" >>"templates/kustomization.yaml"
      echo "- kustomizeversions.yaml" >>"templates/kustomization.yaml"
    fi
  fi
}

# up a cluster with kind
create_cluster() {
  # actually create the cluster
  KIND_IS_UP=true

  tracestate="$(shopt -po xtrace)"
  set +o xtrace

  if [[ -n ${USE_CI_ARTIFACTS:-} ]]; then
    # TODO: revert to https://dl.k8s.io/ci/latest-green.txt once https://github.com/kubernetes/release/issues/897 is fixed.
    CI_VERSION=${CI_VERSION:-$(curl -sSL https://dl.k8s.io/ci/k8s-master.txt)}
  fi

  # Load the newly built image into kind and start the cluster
  (UCLOUD_REGION=${UCLOUD_REGION} \
    UCLOUD_PROJECT=${UCLOUD_PROJECT} \
    CONTROL_PLANE_MACHINE_COUNT=1 \
    WORKER_MACHINE_COUNT=2 \
    KUBERNETES_VERSION=${KUBERNETES_VERSION} \
    UCLOUD_CONTROL_PLANE_MACHINE_TYPE=n1-standard-2 \
    UCLOUD_NODE_MACHINE_TYPE=n1-standard-2 \
    UCLOUD_NETWORK_NAME=${UCLOUD_NETWORK_NAME} \
    CLUSTER_NAME="${CLUSTER_NAME}" \
    CI_VERSION="${CI_VERSION:-}" \
    LOAD_IMAGE="uhub.service.ucloud.cn/${UCLOUD_PROJECT}/cluster-api-ucloud-controller-amd64:dev" \
    make create-cluster)

  eval "$tracestate"

  # Wait till all machines are running (bail out at 30 mins)
  attempt=0
  while true; do
    kubectl get machines --context=kind-clusterapi
    read running total <<<$(kubectl get machines --context=kind-clusterapi \
      -o json | jq -r '.items[].status.phase' | awk 'BEGIN{count=0} /(r|R)unning/{count++} END{print count " " NR}')
    if [[ $total == "3" && $running == "3" ]]; then
      return 0
    fi
    read failed total <<<$(kubectl get machines --context=kind-clusterapi \
      -o json | jq -r '.items[].status.phase' | awk 'BEGIN{count=0} /(f|F)ailed/{count++} END{print count " " NR}')
    if [[ ! $failed -eq 0 ]]; then
      echo "$failed machines (out of $total) in cluster failed ... bailing out"
      exit 1
    fi
    timestamp=$(date +"[%H:%M:%S]")
    if [ $attempt -gt 180 ]; then
      echo "cluster did not start in 30 mins ... bailing out!"
      exit 1
    fi
    echo "$timestamp Total machines : $total / Running : $running .. waiting for 10 seconds"
    sleep 10
    attempt=$((attempt + 1))
  done
}

# run e2es with kubetest
run_tests() {
  # export the KUBECONFIG
  KUBECONFIG="${PWD}/kubeconfig"
  export KUBECONFIG

  # ginkgo regexes
  SKIP="${SKIP:-}"
  FOCUS="${FOCUS:-"\\[Conformance\\]"}"
  # if we set PARALLEL=true, skip serial tests set --ginkgo-parallel
  if [[ "${PARALLEL:-false}" == "true" ]]; then
    export GINKGO_PARALLEL=y
    if [[ -z "${SKIP}" ]]; then
      SKIP="\\[Serial\\]"
    else
      SKIP="\\[Serial\\]|${SKIP}"
    fi
  fi

  # get the number of worker nodes
  # TODO(bentheelder): this is kinda gross
  NUM_NODES="$(kubectl get nodes --kubeconfig="$KUBECONFIG" \
    -o=jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.taints}{"\n"}{end}' |
    grep -cv "node-role.kubernetes.io/master")"

  # wait for all the nodes to be ready
  kubectl wait --for=condition=Ready node --kubeconfig="$KUBECONFIG" --all || true

  # setting this env prevents ginkg e2e from trying to run provider setup
  export KUBERNETES_CONFORMANCE_TEST="y"
  # run the tests
  (cd "$(go env GOPATH)/src/k8s.io/kubernetes" && ./hack/ginkgo-e2e.sh \
    '--provider=skeleton' "--num-nodes=${NUM_NODES}" \
    "--ginkgo.focus=${FOCUS}" "--ginkgo.skip=${SKIP}" \
    "--report-dir=${ARTIFACTS}" '--disable-log-dump=true')

  unset KUBECONFIG
  unset KUBERNETES_CONFORMANCE_TEST
}

# setup kind, build kubernetes, create a cluster, run the e2es
main() {
  for arg in "$@"; do
    if [[ "$arg" == "--verbose" ]]; then
      set -o xtrace
    fi
    if [[ "$arg" == "--clean" ]]; then
      cleanup
      return 0
    fi
    if [[ "$arg" == "--use-ci-artifacts" ]]; then
      USE_CI_ARTIFACTS="1"
    fi
  done

  if [[ -z "$UCLOUD_ACCESS_PUBKEY" ]]; then
    cat <<EOF
$UCLOUD_ACCESS_PUBKEY is not set.
EOF
    return 2
  fi
  if [[ -z "$UCLOUD_ACCESS_PRIKEY" ]]; then
    cat <<EOF
$UCLOUD_ACCESS_PRIKEY is not set.
EOF
    return 2
  fi
  if [[ -z "$UCLOUD_PROJECT_ID" ]]; then
    cat <<EOF
$UCLOUD_PROJECT_ID is not set.
EOF
    return 2
  fi
  if [[ -z "$UCLOUD_PROJECT" ]]; then
    cat <<EOF
$UCLOUD_PROJECT is not set.
EOF
    return 2
  fi
  if [[ -z "$SSH_PASSWORD" ]]; then
    cat <<EOF
$SSH_PASSWORD is not set.
EOF
    return 2
  fi
  if [[ -z "$CLUSTER_NAME" ]]; then
    export CLUSTER_NAME="test"
    cat <<EOF
$CLUSTER_NAME is not set. Using default cluster name: test .
EOF
  fi
  if [[ -z "$KUBERNETES_VERSION" ]]; then
    export KUBERNETES_VERSION="v1.18.1"
    cat <<EOF
$KUBERNETES_VERSION is not set. Using default cluster name: v1.18.1 .
EOF
  fi

  # create temp dir and setup cleanup
  TMP_DIR=$(mktemp -d)
  SKIP_CLEANUP=${SKIP_CLEANUP:-""}
  if [[ -z "${SKIP_CLEANUP}" ]]; then
    trap exit-handler EXIT
  fi
  # ensure artifacts exists when not in CI
  export ARTIFACTS
  mkdir -p "${ARTIFACTS}/logs"

  source "${REPO_ROOT}/hack/ensure-go.sh"
  source "${REPO_ROOT}/hack/ensure-kind.sh"

  # now build and run the cluster and tests
  build
  generate_manifests
  create_cluster

  if [[ -z "${SKIP_RUN_TESTS:-}" ]]; then
    run_tests
  fi
}

main "$@"
