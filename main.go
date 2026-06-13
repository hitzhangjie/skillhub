package main

import (
	"github.com/hitzhangjie/skillhub/cmd"
	"github.com/hitzhangjie/skillhub/pkg/config"
)

var Version = "dev"

func main() {
	config.Version = Version
	cmd.Execute()
}
