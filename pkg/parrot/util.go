package parrot

import (
	"fmt"
	"os"
)

func (p *Parrot) handleError(err error) {
	if err != nil {
		fmt.Println(fmt.Sprintf("An error occured: %v\n", err))
		os.Exit(-1)
	}
}
