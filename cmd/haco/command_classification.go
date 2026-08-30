package main

import "sort"

type commandDomain string

const (
	commandDomainGeneral      commandDomain = "general-controller-client"
	commandDomainPhysicalHost commandDomain = "physical-host-local"
	commandDomainTrustedHost  commandDomain = "trusted-haco-host-local"
	commandDomainCompatibility commandDomain = "temporary-compatibility"
)

type commandClassification struct {
	Name        string
	Domain      commandDomain
	State       string
	Replacement string
}

// hacoCommandClassifications is the authoritative Phase 3 responsibility audit
// for historical runtime-facing top-level haco commands. Standalone support
// commands such as help/version are intentionally handled before runtime
// composition and are not part of this migration table.
var hacoCommandClassifications = map[string]commandClassification{
	"env": {
		Name:   "env",
		Domain: commandDomainGeneral,
		State:  "implemented over the controller client",
	},
	"base": {
		Name:   "base",
		Domain: commandDomainGeneral,
		State:  "controller migration pending (#333)",
	},
	"run": {
		Name:   "run",
		Domain: commandDomainGeneral,
		State:  "controller migration pending (#333)",
	},
	"events": {
		Name:   "events",
		Domain: commandDomainGeneral,
		State:  "controller migration pending (#333)",
	},
	"capability": {
		Name:   "capability",
		Domain: commandDomainGeneral,
		State:  "controller migration pending (#333)",
	},
	"connections": {
		Name:   "connections",
		Domain: commandDomainGeneral,
		State:  "controller/session migration pending (#334)",
	},
	"forward": {
		Name:   "forward",
		Domain: commandDomainGeneral,
		State:  "controller/session migration pending (#334)",
	},
	"unforward": {
		Name:   "unforward",
		Domain: commandDomainGeneral,
		State:  "controller/session migration pending (#334)",
	},
	"ssh": {
		Name:   "ssh",
		Domain: commandDomainGeneral,
		State:  "controller/session migration pending (#334)",
	},
	"host": {
		Name:   "host",
		Domain: commandDomainPhysicalHost,
		State:  "implemented Physical Host bootstrap/recovery lifecycle",
	},
	"doctor": {
		Name:   "doctor",
		Domain: commandDomainPhysicalHost,
		State:  "current Physical Host runtime/backend diagnostics",
	},
	"egress": {
		Name:   "egress",
		Domain: commandDomainPhysicalHost,
		State:  "Physical Host managed egress service (egress serve)",
	},
	"plugin": {
		Name:   "plugin",
		Domain: commandDomainTrustedHost,
		State:  "Git/OCI trusted Host-local migration pending (#335)",
	},
	"create": {
		Name:        "create",
		Domain:      commandDomainCompatibility,
		State:       "legacy Environment spelling",
		Replacement: "haco env create",
	},
	"status": {
		Name:        "status",
		Domain:      commandDomainCompatibility,
		State:       "legacy Environment spelling",
		Replacement: "haco env status",
	},
	"exec": {
		Name:        "exec",
		Domain:      commandDomainCompatibility,
		State:       "legacy Environment spelling",
		Replacement: "haco env exec",
	},
	"shell": {
		Name:        "shell",
		Domain:      commandDomainCompatibility,
		State:       "legacy Environment spelling",
		Replacement: "haco env shell",
	},
	"delete": {
		Name:        "delete",
		Domain:      commandDomainCompatibility,
		State:       "legacy Environment spelling",
		Replacement: "haco env delete",
	},
}

var historicalHacoCommands = []string{
	"env",
	"create",
	"base",
	"plugin",
	"run",
	"events",
	"capability",
	"egress",
	"status",
	"connections",
	"forward",
	"unforward",
	"ssh",
	"exec",
	"shell",
	"delete",
	"doctor",
	"host",
}

func hacoCommandClassification(command string) (commandClassification, bool) {
	classification, ok := hacoCommandClassifications[command]
	return classification, ok
}

func commandClassificationsByDomain(domain commandDomain) []commandClassification {
	result := make([]commandClassification, 0)
	for _, classification := range hacoCommandClassifications {
		if classification.Domain == domain {
			result = append(result, classification)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}
