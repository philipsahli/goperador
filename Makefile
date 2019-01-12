.PHONY: build deploy
THIS_FILE := $(lastword $(MAKEFILE_LIST))

SDK=operator-sdk
build:
	$(SDK) generate k8s
	$(SDK) build $$(minishift openshift registry)/gontador/goperador

crd: build 
	kubectl apply -f deploy/crds/app_v1alpha1_gontadorservice_crd.yaml

operator: crd
	kubectl apply -f deploy/operator.yaml

push: 
	docker push 172.30.1.1:5000/gontador/goperador

cr: 
	kubectl apply -f deploy/crds/app_v1alpha1_gontadorservice_sun.yaml
	kubectl apply -f deploy/crds/app_v1alpha1_gontadorservice_rain.yaml

test: deploy
	kubectl apply -f deploy/crds/app_v1alpha1_gontadorservice_cr.yaml

delete:
	oc delete crd gontadorservices.app.example.com
	oc delete deployment operator

local: build
	operator-sdk up local --namespace gontador

all: build push crd operator
