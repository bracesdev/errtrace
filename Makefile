# only use -race if NO_RACE is unset.
RACE=$(if $(NO_RACE),,-race)

.PHONY: test
test:
	go test $(RACE) -v ./...
	go test $(RACE) -tags safe -v ./...

.PHONY: bench
bench:
	go test -run NONE -bench . -cpu 1

.PHONY: bench-parallel
bench-parallel:
	go test -run NONE -bench . -cpu 1,2,4,8

