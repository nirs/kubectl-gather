e2e:
	go build -o $@ ./cmd

clusters: e2e
	./e2e create

test: clusters
	go test . -v -count=1

clean: e2e
	./e2e delete
	rm -rf out
	rm -f e2e
