// Command pclink2vd converts PCLink (LINK G4X) CSV log exports into
// Virtual Dyno compatible CSV logs.
//
// Usage:
//
//	pclink2vd                 start the GUI
//	pclink2vd -h              show CLI options
package main

import (
	"os"
)

func main() {
	if len(os.Args) > 1 {
		os.Exit(runCLI(os.Args[1:]))
	}
	runGUI()
}
