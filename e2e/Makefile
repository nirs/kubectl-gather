e2e:
	go build -o $@ ./cmd

clusters: e2e
	./e2e create

test: clusters
	go test . -v -count=1

clean:
	./e2e delete
	rm -rf test-*.out
	rm -f kind-*.yaml
	rm -f clusters.yaml
	rm -f e2e
