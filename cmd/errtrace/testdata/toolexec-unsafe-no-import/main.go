package main

import "fmt"

func main() {
	if err := getErr(); err != nil {
		fmt.Printf("%+v\n", err)
	}
}

func getErr() error {
	return fmt.Errorf("err")
}
