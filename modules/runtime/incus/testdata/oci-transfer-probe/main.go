package main

import (
	"fmt"
	"os"
)

func main() {
	const value = "hacocoon-transfer-persisted"
	data, err := os.ReadFile("/persisted")
	if os.IsNotExist(err) {
		file, err := os.OpenFile("/persisted", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			os.Exit(1)
		}
		if _, err = file.WriteString(value); err != nil {
			os.Exit(1)
		}
		if err = file.Sync(); err != nil {
			os.Exit(1)
		}
		if err = file.Close(); err != nil {
			os.Exit(1)
		}
		fmt.Println("created")
		return
	}
	if err != nil || string(data) != value {
		os.Exit(1)
	}
	fmt.Println("retained")
}
