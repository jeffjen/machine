package ssh

import (
	"github.com/codegangsta/cli"

	"fmt"
	"path"
	"strings"
)

func parseArgs(c *cli.Context) (user, key string, hosts []string) {
	return c.String("user"), c.String("cert"), c.StringSlice("host")
}

func runCmd(c *cli.Context) {
	var (
		cmd              = strings.Join(c.Args(), " ")
		collect          = make(chan error)
		user, key, hosts = parseArgs(c.Parent())
	)
	for _, host := range hosts {
		go func(host string) {
			sshctx := New(Config{User: user, Server: host, Key: key, Port: "22"})
			resp, err := sshctx.Run(cmd)
			if err != nil {
				fmt.Println(err.Error())
				collect <- err
			} else {
				fmt.Println(host, "-", resp)
				collect <- nil
			}
		}(host)
	}
	for chk := 0; chk < len(hosts); chk++ {
		<-collect
	}
}

func runScript(c *cli.Context) {
	var (
		scripts          = c.Args()
		collect          = make(chan error)
		sudo             = c.Parent().Bool("sudo")
		user, key, hosts = parseArgs(c.Parent())
	)
	for _, host := range hosts {
		go func(host string) {
			var (
				respStream <-chan Response
				err        error

				text string
			)
			sshctx := New(Config{User: user, Server: host, Key: key, Port: "22"})
			for _, script := range scripts {
				dst := path.Join("/tmp", path.Base(script))
				if err = sshctx.CopyFile(script, dst, 0644); err != nil {
					fmt.Println(err.Error())
					collect <- err
					return
				}
				fmt.Println(host, "- sent script", script, "->", dst)
				if sudo {
					respStream, err = sshctx.Sudo().Stream("bash " + dst)
				} else {
					respStream, err = sshctx.Stream("bash " + dst)
				}
				if err != nil {
					fmt.Println(err.Error())
					collect <- err
					return
				}
				for output := range respStream {
					text, err = output.Data()
					if err != nil {
						fmt.Println(host, "-", err.Error())
						// steam will end because error state delivers last
					} else {
						fmt.Println(host, "-", text)
					}
				}
				if err != nil { // abort execution if script failed
					collect <- err
					return
				}
			}
			collect <- nil // mark end of script run
		}(host)
	}
	for chk := 0; chk < len(hosts); chk++ {
		<-collect
	}
}

func NewCommand() cli.Command {
	return cli.Command{
		Name:  "exec",
		Usage: "Invoke command on remote host via SSH",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "user", EnvVar: "MACHINE_USER", Usage: "Run command as user"},
			cli.StringFlag{Name: "cert", EnvVar: "MACHINE_CERT_FILE", Usage: "Private key to use in Authentication"},
			cli.StringSliceFlag{Name: "host", Usage: "Remote host to run command in"},
			cli.BoolFlag{Name: "sudo", Usage: "Run as sudo for this session"},
		},
		Subcommands: []cli.Command{
			{
				Name:   "run",
				Usage:  "Invoke command from argument",
				Action: runCmd,
			},
			{
				Name:   "script",
				Usage:  "Invoke script from argument",
				Action: runScript,
			},
		},
	}
}
