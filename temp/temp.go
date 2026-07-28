package main

import (
	"fmt"
	"time"
)

func main() {
	now := time.Now()

	fmt.Println(now.Date())

	// var today string

	// year, month, date := now.Date()

	today := now.Format("20060102")
	fmt.Println(today)
}
