package clientforward

import "github.com/SLktEx/Hacocoon/internal/cliui"

// HelpOptions describes the shared parser for both maintained client entries.
func HelpOptions() []cliui.HelpField {
	return []cliui.HelpField{
		{Syntax: "--target-port <port>", Message: "detail.target_port"},
		{Syntax: "--address <loopback-ip>", Message: "forward.address"},
		{Syntax: "--listen <loopback-ip:port>", Message: "forward.listen"},
		{Syntax: "--duration <duration>", Message: "forward.duration"},
	}
}
