package dnsproxy

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	logs "github.com/jursonmo/beelogs"
	radixTrie "github.com/jursonmo/go-radix"
	"github.com/miekg/dns"
)

type Trier interface {
	InsertDomain(name string, data interface{}) (interface{}, bool)
	DeleteDomain(name string) (interface{}, bool)
	FindDomain(name string) (interface{}, error)
}

type PolicyConfig struct {
	Path  string   `toml:"path"`
	Files []string `toml:"files"`
}

type policyValue struct {
	domain  string
	ipset   []string
	script  string
	server  string
	address string
}

type Policy struct {
	path  string
	files []string
	tree  Trier
}

func NewPolicy(cfg *PolicyConfig) *Policy {
	return &Policy{
		path:  cfg.Path,
		files: cfg.Files,
		tree:  radixTrie.New(),
	}
}

func (p *Policy) Load() {
	dir, err := ioutil.ReadDir(p.path)
	if err == nil {
		for _, file := range dir {
			if file.IsDir() {
				continue
			}

			p.files = append(p.files, fmt.Sprintf("%s/%s", p.path, file.Name()))
		}
	}

	for _, file := range p.files {
		p.loadfile(file)
	}
}

func (p *Policy) loadfile(file string) {
	fp, err := os.Open(file)
	if err != nil {
		logs.Warn("open file:%s, fail: %v", file, err)
		return
	}
	logs.Info("load config :%s", file)
	br := bufio.NewReader(fp)
	for {
		bline, _, err := br.ReadLine()
		if err != nil {
			break
		}

		p.loadline(string(bline))
	}
}

func (p *Policy) loadline(line string) {
	line = strings.TrimLeft(line, " ")

	if strings.HasPrefix(line, "#") {
		return
	}

	// server=/whatsapp.com/8.8.8.8#53
	// ipset=/whatsapp.com/US-DNS,US-DNSv6
	// script=/whatsapp.com//data/dnsproxy/route.sh
	// address=/whatsapp.com/192.168.4.157
	sp := strings.Split(line, "/")
	if len(sp) < 3 {
		logs.Warn("invalid line:%s", line)
		return
	}

	plugin := sp[0]
	domain := fmt.Sprintf("*.%s", sp[1]) //add wildcard, baidu.com-->*.baidu.com, www.baidu.com will match this, but wwwbaidu.com not match

	// plugin为script时，策略可能包含路径的/
	policy := strings.Join(sp[2:], "/")

	logs.Debug("plugin:%s, domain:%s, policy:%s\n", plugin, domain, policy)
	switch plugin {
	case "server=":
		ele, err := p.tree.FindDomain(domain)
		if err != nil {
			ele = &policyValue{}
		}

		ele.(*policyValue).server = strings.Replace(policy, "#", ":", -1)
		ele.(*policyValue).domain = domain

		p.tree.InsertDomain(domain, ele)

	case "ipset=":
		ele, err := p.tree.FindDomain(domain)
		if err != nil {
			ele = &policyValue{}
		}

		ele.(*policyValue).ipset = strings.Split(policy, ",")
		ele.(*policyValue).domain = domain

		p.tree.InsertDomain(domain, ele)

	case "script=":
		ele, err := p.tree.FindDomain(domain)
		if err != nil {
			ele = &policyValue{}
		}

		ele.(*policyValue).script = policy
		ele.(*policyValue).domain = domain

		p.tree.InsertDomain(domain, ele)

	case "address=":
		// ele, err := p.tree.Find(domain)
		// if err != nil {
		// 	ele = &policyValue{}
		// }

		// ele.(*policyValue).address = policy
		// ele.(*policyValue).domain = domain

		ele := &policyValue{address: policy, domain: domain}
		p.tree.InsertDomain(domain, ele)
	}
}

func (p *Policy) Exec(domain string, resp *dns.Msg) {
	ele, err := p.FindDomain(domain)
	if err != nil {
		return
	}

	val := ele.(*policyValue)

	ips := make([]string, 0)
	for _, a := range resp.Answer {
		hdr := a.Header()

		switch hdr.Rrtype {
		case dns.TypeA:
			a, ok := a.(*dns.A)
			if !ok {
				continue
			}

			ips = append(ips, a.A.String())

		case dns.TypeAAAA:
			aaaa, ok := a.(*dns.AAAA)
			if !ok {
				continue
			}

			ips = append(ips, aaaa.AAAA.String())
		}
	}

	if len(val.ipset) > 0 {
		for _, setname := range val.ipset {
			p.execIpset(ips, setname)
		}
	}

	if val.script != "" {
		p.execScript(ips, val.script)
	}
}

func (p *Policy) execIpset(ips []string, set string) {
	for _, ip := range ips {
		p.execCmd("ipset", []string{"add", set, ip})
	}
}

func (p *Policy) execScript(ips []string, script string) {
	for _, ip := range ips {
		p.execCmd("/bin/bash", []string{"-c", fmt.Sprintf("%s %s", script, ip)})
	}
}

func (p *Policy) execCmd(cmd string, args []string) {
	exec.Command(cmd, args...).CombinedOutput()
}

func (p *Policy) GetUpper(domain string) []string {
	ele, err := p.FindDomain(domain)
	if err != nil {
		return nil
	}

	srv := ele.(*policyValue).server
	if srv == "" {
		return nil
	}

	return []string{srv}
}

func (p *Policy) GetAddress(domain string) []string {
	ele, err := p.FindDomain(domain)
	if err != nil {
		return nil
	}

	addr := ele.(*policyValue).address
	if addr == "" {
		return nil
	}

	return []string{addr}
}

func (p *Policy) FindDomain(domain string) (interface{}, error) {	
	return p.tree.FindDomain("."+domain)
}