package main

import (
	"fmt"

	"github.com/talkersoft/hive-deck/internal/namegen"
)

func main() {
	for i := range 50 {
		fmt.Printf("%2d. %s\n", i+1, namegen.Generate())
	}
}
