# Current Operator version
# “？=”表示如果该变量没有被赋值，则赋予等号后的值
VERSION ?= 0.13.0
# Default bundle image tag
BUNDLE_IMG ?= controller-bundle:$(VERSION)
# Options for 'bundle-build'
# ifeq和ifneq在逻辑上就是C语言的if语句，ifeq是后面的条件成立时执行，ifneq是条件不成立时执行，具体用哪一个根据需求。
ifneq ($(origin CHANNELS), undefined)
# 进到这里，说明CHANNELS变量没定义
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

# Image URL to use all building/pushing image targets
IMG ?= registry.cn-hangzhou.aliyuncs.com/leijianzhong/redis-operator:$(VERSION)
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true,preserveUnknownFields=false"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
# GOBIN=/root/go/bin
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: manager

# Run tests
ENVTEST_ASSETS_DIR=$(shell pwd)/testbin
test: generate fmt vet manifests
	mkdir -p ${ENVTEST_ASSETS_DIR}
	test -f ${ENVTEST_ASSETS_DIR}/setup-envtest.sh || curl -sSLo ${ENVTEST_ASSETS_DIR}/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/v0.7.0/hack/setup-envtest.sh
	source ${ENVTEST_ASSETS_DIR}/setup-envtest.sh; fetch_envtest_tools $(ENVTEST_ASSETS_DIR); setup_envtest_env $(ENVTEST_ASSETS_DIR); go test ./... -coverprofile cover.out

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests
	go run ./main.go

# Install CRDs into a cluster
install: manifests kustomize
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests kustomize
	$(KUSTOMIZE) build config/crd | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests kustomize
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

# UnDeploy controller from the configured Kubernetes cluster in ~/.kube/config
undeploy:
	$(KUSTOMIZE) build config/default | kubectl delete -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
# Go vet 命令在编写代码时非常有用。它可以帮助您检测应用程序中任何可疑、异常或无用的代码。
vet:
	go vet ./...

# Generate code
# paths="./..." 代表路径和go-style路径模式，作为包根使用。
# 如果没有手动指定 fallback 规则，默认的 per-generator 将被使用，生成的 YAML 将放到 config/<generator>相应目录，代码所在的位置不变
# 下面的 controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./..." 命令用于生成生成zz_xxx.deepcopy.go文件
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Build the docker image
docker-build:
	docker build -t ${IMG} .

# Push the docker image
docker-push:
	docker push ${IMG}

# Download controller-gen locally if necessary
# 安装controller-gen controller-gen可以生成zz_xxx.deepcopy.go文件以及crd文件
CONTROLLER_GEN = $(shell pwd)/bin/controller-gen
controller-gen:
	$(call go-get-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@v0.4.1)

# Download kustomize locally if necessary
# 如果你经常使用 Kubernetes，那么你肯定就有定制资源清单文件的需求，但是貌似现在大家都比较喜欢使用 Helm。
# Helm 很好用，但也有很多缺点，比如需要一个 tiller 服务端，需要超高的权限，最重要的是如果你要想自己做一个 Helm Chart 包的话，则不是那么容易的，需要你了解一些 go template 的相关知识，它抛弃了我们在 Docker 和 Kubernetes 上面学到的一些逻辑，
#  另外一种名为Kustomize的替代工具。
KUSTOMIZE = $(shell pwd)/bin/kustomize
kustomize:
	$(call go-get-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v3@v3.8.7)

# $(MAKEFILE_LIST) => Makefile
# $(lastword $(MAKEFILE_LIST)) => Makefile
# $(abspath $(lastword $(MAKEFILE_LIST))) => /root/redis-operator/Makefile
# dirname $(abspath $(lastword $(MAKEFILE_LIST))) /root/redis-operator
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
# go-get-tool will 'go get' any package $2 and install it to $1.
# 定义一个自定义函数
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin go get $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef

# Generate bundle manifests and metadata, then validate generated files.
.PHONY: bundle
bundle: manifests kustomize
	operator-sdk generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | operator-sdk generate bundle -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)
	operator-sdk bundle validate ./bundle

# Build the bundle image.
.PHONY: bundle-build
bundle-build:
	docker build -f bundle.Dockerfile -t $(BUNDLE_IMG) .
