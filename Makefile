# only use -race if NO_RACE is unset.
RACE=$(if $(NO_RACE),,-race)

.PHONY: test
test:
	go test $(RACE) -v ./...
	go test $(RACE) -tags safe -v ./...

