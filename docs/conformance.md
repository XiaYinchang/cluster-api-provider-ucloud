# Running Conformance tests

## Required environment variables
- Set the UCLOUD region
```
export UCLOUD_REGION=cn-bj2
```
- Set the UCLOUD project to use
```
export UCLOUD_PROJECT_ID=your-project-id
```
- Set the credential for UCloud account
```
export UCLOUD_ACCESS_PUBKEY="UCLOUD_ACCESS_PUBKEY"
export UCLOUD_ACCESS_PRIKEY="UCLOUD_ACCESS_PRIKEY"
```
- Set base64 encoded ssh password for hosts
```
export SSH_PASSWORD="Y2x1c3Rlci1hcGk="
```

## Optional environment variables
- Set kubernetes version
```
export KUBERNETES_VERSION=v1.18.1
```
- Set a specific name for your cluster
```
export CLUSTER_NAME=test1
```
- Skip running tests
```
export SKIP_RUN_TESTS=1
```
- Skip cleaning up the project resources
```
export SKIP_CLEANUP=1
```

## Running the conformance tests
```
hack/ci/e2e-conformance.sh
```

## How to cleanup if you used SKIP_CLEANUP to start hack/ci/e2e-conformance.sh earlier
```
hack/ci/e2e-conformance.sh --cleanup
```

## Gimme Moar! logs!!!
```
hack/ci/e2e-conformance.sh --verbose
```
