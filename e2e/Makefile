deploy undeploy: export KUBECONFIG = out/kubeconfig.yaml

e2e:
	go build -o $@ ./cmd

test: clusters deploy
	go test . -v -count=1

clusters: e2e
	./e2e create

deploy:
	# Deploy common deployment on both clusters.
	kubectl apply -k testdata/common --context kind-c1
	kubectl apply -k testdata/common --context kind-c2
	# Deploy c1 deployment on c1.
	kubectl apply -k testdata/c1 --context kind-c1
	# Deploy c2 deployment on c2.
	kubectl apply -k testdata/c2 --context kind-c2
	# Wait for all deployments.
	kubectl rollout status deploy common-busybox -n test-common --context kind-c1
	kubectl rollout status deploy common-busybox -n test-common --context kind-c2
	kubectl rollout status deploy c1-busybox -n test-c1 --context kind-c1
	kubectl rollout status deploy c2-busybox -n test-c2 --context kind-c2

undeploy:
	# Delete common deployment on both clusters.
	kubectl delete -k testdata/common --context kind-c1 --ignore-not-found --wait=false
	kubectl delete -k testdata/common --context kind-c2 --ignore-not-found --wait=false
	# Delete c1 deployment on c1.
	kubectl delete -k testdata/c1 --context kind-c1 --ignore-not-found --wait=false
	# Delete c2 deployment on c2.
	kubectl delete -k testdata/c2 --context kind-c2 --ignore-not-found --wait=false
	# Wait for all deletions.
	kubectl wait ns test-common --for delete --context kind-c1
	kubectl wait ns test-common --for delete --context kind-c2
	kubectl wait ns test-c1 --for delete --context kind-c1
	kubectl wait ns test-c2 --for delete --context kind-c2

clean: e2e
	./e2e delete
	rm -rf out
	rm -f e2e
