// Pulsewise shows the Mac's CPU, GPU and memory in the menu bar.
package main

import (
	"flag"
	"log"

	"github.com/FriendlyPasser/pulsewise/internal/gui"
)

func main() {
	webAddr := flag.String("web", "", "serve the panel to a browser at this address (e.g. 127.0.0.1:8765) instead of the menu bar")
	flag.Parse()
	run := func() error { return gui.Run() }
	if *webAddr != "" {
		run = func() error { return gui.ServeWeb(*webAddr) }
	}
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
