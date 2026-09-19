package bench

import (
	"fmt"
	"strings"
)

// Fixtures are bench-owned documents, generated deterministically so a read
// task fully controls what the agent can see. They use invented names and
// values, so an agent cannot answer from prior knowledge.

var fixtureNames = []string{
	"ashgrove", "brindle", "cindermoth", "dunlin", "emberwick", "fallowmere", "gorsehaven", "hollowpine",
	"ivyclad", "juniperfen", "kelpstone", "larkspur", "marrowdale", "nettlebrook", "oakmoss", "pebblehithe",
	"quillfox", "rookwater", "saltmarrow", "tarnwick", "umberfield", "veldtmoor", "willowrake", "yarrowick",
	"zephyrnook", "amberlock", "burrowgate", "cobblefern", "duskthistle", "eldermarch", "flintcombe", "glenharrow",
	"heathermill", "irongrass", "jaspervale", "kestrelbay", "lichenford", "mistlethorn", "noctuary", "ospreyhill",
	"pinewhistle", "quartzden", "ravenmoor", "sedgewick", "thistledown", "umbercrest", "vesperlow", "wrenfield",
	"yewbridge", "zinniafold", "acornreach", "bracken", "cairnwell", "dovecote", "elmstead", "foxglove",
}

// Needle values the bench tasks ask about. Filler values never collide with them.
const (
	needleService = "quillfox"
	needleRetry   = 17
	needlePort    = 7431
	needleOwner   = "Bramble"
	needlePortSvc = "tarnwick"
)

var owners = []string{"Cedar", "Heron", "Sable", "Marlin", "Thorn", "Lantern", "Quartz", "Willow"}

// fixtureDoc returns the named fixture document.
func fixtureDoc(name string) (string, bool) {
	if name != "RUNBOOK.md" {
		return "", false
	}
	var b strings.Builder
	b.WriteString("# Fenwick Platform Runbook\n\n")
	b.WriteString("Service reference for the Fenwick platform. Values are per service; defaults apply where a line is absent.\n\n")
	for i, svc := range fixtureNames {
		port, retry, owner := 7000+13*i, 3+(i*5)%11, owners[i%len(owners)]
		if svc == needlePortSvc {
			port, owner = needlePort, needleOwner
		}
		if svc == needleService {
			retry = needleRetry
		}
		fmt.Fprintf(&b, "## %s\n\n", svc)
		fmt.Fprintf(&b, "- role: %s worker for the %s ledger\n", []string{"queue", "batch", "stream"}[i%3], svc)
		fmt.Fprintf(&b, "- port: %d\n", port)
		fmt.Fprintf(&b, "- retry limit: %d attempts before the message is parked\n", retry)
		fmt.Fprintf(&b, "- timeout: %d seconds per attempt, doubled after every second retry\n", 20+(i*7)%40)
		fmt.Fprintf(&b, "- pager rotation: Team %s (handover every Monday 09:00 UTC)\n", owner)
		fmt.Fprintf(&b, "- health probe: GET /healthz/%s answers 200 within %d ms when the service is healthy\n", svc, 50+(i*11)%90)
		fmt.Fprintf(&b, "- rollout: canary %d%% for %d minutes, then a linear ramp; abort on any parked message\n", 5+(i*3)%20, 10+(i*4)%30)
		fmt.Fprintf(&b, "- storage: %d GiB volume, snapshot every %d hours, retained for %d days\n", 40+(i*9)%200, 2+(i*3)%10, 7+(i*5)%40)
		fmt.Fprintf(&b, "- dashboards: fenwick/%s/latency and fenwick/%s/errors\n", svc, svc)
		fmt.Fprintf(&b, "- escalation: page the rotation first, then the platform lead after %d minutes\n\n", 10+(i*3)%20)
	}
	return b.String(), true
}
