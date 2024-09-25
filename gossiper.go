package gossiper

import (
	"github.com/fatih/color"
	"github.com/pieceowater-dev/lotof.lib.gossiper/environment"
	"log"
)

func Setup() {
	color.Set(color.FgGreen)
	log.SetFlags(log.LstdFlags)
	log.Println("Setting up Gossiper...")

	environment.Init()

	color.Set(color.FgCyan)
	log.Println("Setup complete.")
}
