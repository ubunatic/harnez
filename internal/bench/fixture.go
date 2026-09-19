package bench

import (
	"fmt"
	"sort"
	"strconv"
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

type service struct {
	name   string
	fields [][2]string // key, value; port is the only numeric value
}

// services builds the fixture data once; every delivery format renders from it.
func services() []service {
	var out []service
	for i, svc := range fixtureNames {
		port, retry, owner := 7000+13*i, 3+(i*5)%11, owners[i%len(owners)]
		if svc == needlePortSvc {
			port, owner = needlePort, needleOwner
		}
		if svc == needleService {
			retry = needleRetry
		}
		out = append(out, service{name: svc, fields: [][2]string{
			{"role", fmt.Sprintf("%s worker for the %s ledger", []string{"queue", "batch", "stream"}[i%3], svc)},
			{"port", fmt.Sprint(port)},
			{"retry limit", fmt.Sprintf("%d attempts before the message is parked", retry)},
			{"timeout", fmt.Sprintf("%d seconds per attempt, doubled after every second retry", 20+(i*7)%40)},
			{"pager rotation", fmt.Sprintf("Team %s (handover every Monday 09:00 UTC)", owner)},
			{"health probe", fmt.Sprintf("GET /healthz/%s answers 200 within %d ms when the service is healthy", svc, 50+(i*11)%90)},
			{"rollout", fmt.Sprintf("canary %d%% for %d minutes, then a linear ramp; abort on any parked message", 5+(i*3)%20, 10+(i*4)%30)},
			{"storage", fmt.Sprintf("%d GiB volume, snapshot every %d hours, retained for %d days", 40+(i*9)%200, 2+(i*3)%10, 7+(i*5)%40)},
			{"dashboards", fmt.Sprintf("fenwick/%s/latency and fenwick/%s/errors", svc, svc)},
			{"escalation", fmt.Sprintf("page the rotation first, then the platform lead after %d minutes", 10+(i*3)%20)},
		}})
	}
	sort.Slice(out, func(a, b int) bool { return out[a].name < out[b].name })
	return out
}

func renderMarkdown(title string, svcs []service) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", title)
	b.WriteString("Service reference for the Fenwick platform. Values are per service; defaults apply where a line is absent.\n\n")
	for _, s := range svcs {
		fmt.Fprintf(&b, "## %s\n\n", s.name)
		for _, f := range s.fields {
			fmt.Fprintf(&b, "- %s: %s\n", f[0], f[1])
		}
		b.WriteString("\n")
	}
	return b.String()
}

func renderYAML(title string, svcs []service) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n", title)
	b.WriteString("services:\n")
	for _, s := range svcs {
		fmt.Fprintf(&b, "  %s:\n", s.name)
		for _, f := range s.fields {
			val := strconv.Quote(f[1])
			if f[0] == "port" {
				val = f[1]
			}
			fmt.Fprintf(&b, "    %s: %s\n", strings.ReplaceAll(f[0], " ", "_"), val)
		}
	}
	return b.String()
}

type fixtureFile struct{ Name, Content string }

// MaxMulti is the largest --multi split: one file per starting letter.
const MaxMulti = 26

// fixtureFiles renders the named fixture as Markdown or YAML, whole or split
// into n files that each cover a contiguous group of first letters (26/n
// letters per file; groups without services are skipped).
func fixtureFiles(name string, yaml bool, multi int) ([]fixtureFile, bool) {
	if name != "RUNBOOK.md" {
		return nil, false
	}
	stem, ext, render := "RUNBOOK", ".md", renderMarkdown
	if yaml {
		ext, render = ".yaml", renderYAML
	}
	all := services()
	const title = "Fenwick Platform Runbook"
	if multi < 2 {
		return []fixtureFile{{stem + ext, render(title, all)}}, true
	}
	per := (26 + multi - 1) / multi
	var out []fixtureFile
	for lo := 0; lo < 26; lo += per {
		hi := min(lo+per, 26) - 1
		var group []service
		for _, s := range all {
			if c := int(s.name[0] - 'a'); c >= lo && c <= hi {
				group = append(group, s)
			}
		}
		if len(group) == 0 {
			continue
		}
		span := string(rune('a' + lo))
		if hi > lo {
			span += "-" + string(rune('a'+hi))
		}
		out = append(out, fixtureFile{fmt.Sprintf("%s-%s%s", stem, span, ext), render(title+", services "+span, group)})
	}
	return out, true
}

// fixtureDoc returns the whole Markdown fixture.
func fixtureDoc(name string) (string, bool) {
	files, ok := fixtureFiles(name, false, 0)
	if !ok {
		return "", false
	}
	return files[0].Content, true
}
