package main

import "fmt"

func multipleValueErrAssignment() (err error) {
	defer func() {
		_, err = fmt.Println("Hello, World!")

		// Handles too few lhs variables
		err = fmt.Println("Hello, World!")

		// Handles too many lhs variables
		_, err, _ = fmt.Println("Hello, World!") // want:"skipping assignment: error is not the last return value"

		// Handles misplaced err
		err, _ = fmt.Println("Hello, World!") // want:"skipping assignment: error is not the last return value"
	}()

	return nil
}
