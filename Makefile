SHELL=/bin/bash

BUILDTIME=$(shell date '+%Y-%m-%dT%T%z')
BUILDHASH=$(shell git rev-parse --verify --short HEAD || echo "")
BUILDUSER=$(shell [ -n "${bamboo_ManualBuildTriggerReason_userName}" ] && \
		   echo ${bamboo_ManualBuildTriggerReason_userName} || \
		   echo $$(whoami))

PROJECTNAME?=$(shell basename $$(git rev-parse --show-toplevel))
PROJECTVERS?=$(shell [ -n "${bamboo_BUILD_VERSION}" ] && \
			 echo ${bamboo_BUILD_VERSION} || \
			 echo "latest")
PROJECTDIR=$(shell pwd)
PROJECTGIT?=github.com/martinbaillie
PROJECTSRC=$(PROJECTDIR)/src/$(PROJECTGIT)/$(PROJECTNAME)

GOLDFLAGS?=-ldflags "-s -w -extld ld -extldflags -static \
			-X 'main.buildTime=$(BUILDTIME)' \
			-X 'main.buildHash=$(BUILDHASH)' \
			-X 'main.buildUser=$(BUILDUSER)' \
			-X 'main.projectName=$(PROJECTNAME)' \
			-X 'main.projectVersion=$(PROJECTVERS)'"
GOOS?=linux darwin
GOARCH?=amd64

# Swagger
SWAGGER_UI_VERSION=2.2.8
SWAGGER_UI_URL=https://github.com/swagger-api/swagger-ui/archive/v$(SWAGGER_UI_VERSION).zip

# Docker
DOCKERIMG?=martinbaillie/rancher-management-service
DOCKERREG?=

.PHONY: dep clean vet lint fmt bench build update docker-clean docker-tag \
	docker-push fetch-swagger-ui layout-swagger-ui swagger-ui

all: dep clean swagger-ui test build

dep:
	@echo ">> dependencies"
	@go get -u "github.com/constabulary/gb/..."
	@go get -u "github.com/jstemmer/go-junit-report"
	@go get -u "github.com/golang/lint/golint"
	@go get -u "github.com/go-swagger/go-swagger/..."
	@go get -u "github.com/jteeuwen/go-bindata/..."
	@go get -u "github.com/elazarl/go-bindata-assetfs/..."

clean:
	@echo ">> cleaning"
	@rm -rf bin pkg *.xml *.zip

generate:
	@echo ">> generating"
	@cd "$(PROJECTSRC)/swagger" \
		&& git add doc.go \
		&& sed -i.bak 's/MAKEFILE_REPLACED_VERSION/$(PROJECTVERS)-$(BUILDHASH)/' doc.go \
		&& rm -f doc.go.bak \
		&& gb generate \
		&& git checkout -- doc.go \
		&& git reset HEAD doc.go &>/dev/null

vet:
	@echo ">> vetting"
	@cd $(PROJECTSRC) && go vet ./...

lint:
	@echo ">> linting"
# Awkward linting to avoid go-bindata-assetfs artefacts
	@failed=0; \
		for d in $$(find "$(PROJECTSRC)" -type d); do \
			for f in $$(find "$$d" -maxdepth 1 -type f -name '*.go' ! -name '*_assetfs.go'); do \
				golint -set_exit_status "$$f"; \
				[ $$? -ne 0 ] && failed=1; \
    		done ; \
		done ; \
		exit $$failed
	
fmt:
	@echo ">> formatting"
	@cd $(PROJECTSRC) && go fmt ./...

test: generate vet lint fmt
	@echo ">> testing"
# This is not perfect.
# `go test` may be a better option here but needs $GOPATH hackery.
# See https://github.com/constabulary/gb/issues/559.
	@gb test all -v -race | tee $(PROJECTDIR)/$(PROJECTNAME).out
	@cat $(PROJECTDIR)/$(PROJECTNAME).out | \
		go-junit-report > $(PROJECTDIR)/$(PROJECTNAME).xml && \
		rm $(PROJECTDIR)/$(PROJECTNAME).out

bench:
	@echo ">> benchmarking"
	@cd $(PROJECTSRC) && go test -v ./... -bench=.

build: generate
	@echo ">> building"
	@for arch in ${GOARCH}; do \
		for os in ${GOOS}; do \
			echo ">>>> $${os}/$${arch}"; \
			env GOOS=$${os} GOARCH=$${arch} gb build ${GOLDFLAGS} all; \
		done; \
	done

update:
	@gb vendor update --all

docker-clean:
	@echo ">> cleaning (docker)"
	@docker rmi $(DOCKERREG)/$(DOCKERIMG)-build &>/dev/null || true

docker-build-prepare: docker-clean
	@echo ">> preparing (docker)"
	@docker build --no-cache \
		--build-arg=http_proxy=${http_proxy} \
		--build-arg=https_proxy=${https_proxy} \
		--build-arg=no_proxy=${no_proxy} \
		-t $(DOCKERREG)$(DOCKERIMG)-build -f Dockerfile_build .

docker-build: docker-build-prepare
	@echo ">> building (docker)"
	@test -f $@.cid && { docker rm -f $$(cat $@.cid) && rm $@.cid; } || true;
	@docker run -t --cidfile="$@.cid" \
		-v "$(PROJECTDIR)":"/go/src/$(PROJECTNAME)" \
		$(DOCKERREG)$(DOCKERIMG)-build
	@docker cp $$(cat $@.cid):/etc/ssl/certs/ca-certificates.crt ${PROJECTDIR}
	@docker stop $$(cat $@.cid)
	@docker rm $$(cat $@.cid)
	@docker build -t $(DOCKERIMG) $(PROJECTDIR)
	@rm $@.cid *.crt

docker-tag:
	@echo ">> tagging (docker)"
	@echo ">>>> $(DOCKERREG)$(DOCKERIMG):$(PROJECTVERS)"
	@docker tag $(DOCKERIMG) $(DOCKERREG)$(DOCKERIMG):$(PROJECTVERS)

docker-push:
	@echo ">> pushing (docker)"
	@echo ">>>> $(DOCKERREG)$(DOCKERIMG):$(PROJECTVERS)"
	@docker push $(DOCKERREG)$(DOCKERIMG):$(PROJECTVERS)

release: docker-build docker-tag docker-push

fetch-swagger-ui:
	@echo ">> fetching swagger ui"
	@curl -OLks $(SWAGGER_UI_URL)
	@echo ">>>> downloaded v$(SWAGGER_UI_VERSION)"

layout-swagger-ui:
	@echo ">> unpacking swagger ui"
	@rm -rf $(PROJECTSRC)/swagger-ui
	$(eval TMP := $(shell mktemp -d))
	@unzip v$(SWAGGER_UI_VERSION).zip -d $(TMP) &>/dev/null
	@mv $(TMP)/swagger-ui-$(SWAGGER_UI_VERSION)/dist $(PROJECTSRC)/swagger-ui
	@rm -rf $(TMP)

swagger-ui: fetch-swagger-ui layout-swagger-ui
