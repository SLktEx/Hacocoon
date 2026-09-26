package cli

import (
	"context"
	"flag"
	"fmt"
	controlapi "github.com/SLktEx/Hacocoon/internal/controller/api"
	"os"
	"time"
)

func runVolume(args []string) int {
	if len(args) == 0 {
		commandHelp(os.Stderr, "volume", cliLanguage())
		return 2
	}
	op := args[0]
	if op == "list" {
		op = "ls"
	}
	if op != "create" && op != "ls" && op != "inspect" && op != "rm" {
		commandHelp(os.Stderr, "volume", cliLanguage())
		return 2
	}
	flags := flag.NewFlagSet("haco volume "+op, flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	machine := flags.Bool("json", false, cliMessage("flag.json"))
	var source string
	if op == "create" {
		flags.StringVar(&source, "container", "", "Copy the Environment Workspace")
	}
	if parseInterspersed(flags, args[1:]) != nil {
		return 2
	}
	n := 1
	if op == "ls" {
		n = 0
	}
	if flags.NArg() != n {
		commandHelp(os.Stderr, "volume "+op, cliLanguage())
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	result, err := controlapi.NewDefaultClient().Volume(ctx, controlapi.VolumeRequest{Operation: op, Name: flags.Arg(0), Container: source})
	if err != nil {
		fmt.Fprintln(os.Stderr, "haco:", err)
		return 1
	}
	if *machine {
		if writeCLIResult(os.Stdout, result, true) != nil {
			return 1
		}
	} else {
		for _, v := range result {
			fmt.Fprintln(os.Stdout, v.Name)
		}
	}
	return 0
}
