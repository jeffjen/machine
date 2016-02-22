package main

import (
	"github.com/jeffjen/machine/driver/aws"
	"github.com/jeffjen/machine/lib/ssh"

	"github.com/codegangsta/cli"

	"os"
)

func main() {
	app := cli.NewApp()
	app.Version = "0.0.1"
	app.Name = "machine"
	app.Usage = "Create/Bootstrap machine to use with Docker engine"
	app.Authors = []cli.Author{
		cli.Author{"Yi-Hung Jen", "yihungjen@gmail.com"},
	}
	app.Commands = []cli.Command{
		aws.NewCommand(),
		ssh.NewCommand(),
	}
	app.Before = nil
	app.Action = nil
	app.Run(os.Args)
}
