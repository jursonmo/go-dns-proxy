package dnsproxy

import "testing"

type LoadCase struct {
	in     string
	domain string
	expect string
}

var p = NewPolicy(&PolicyConfig{})

func TestLoadServer(t *testing.T) {
	for _, c := range []LoadCase{
		LoadCase{
			in:     "server=/www.baidu.tech/7.7.7.7#53",
			domain: "www.baidu.tech",
			expect: "7.7.7.7:53",
		},
		LoadCase{
			in:     "server=/mydomain.tech/8.8.8.8#53",
			domain: "www.mydomain.tech",
			expect: "8.8.8.8:53",
		},
	} {
		p.loadline(c.in)
		upper := p.GetUpper(c.domain)
		if len(upper) <= 0 {
			t.Error("expect upper not nil, got nil")
			return
		}

		if upper[0] != c.expect {
			t.Errorf("expect upper %s, got %s\n", c.expect, upper[0])
			return
		}

		t.Logf("domain: %s upper: %v\n", c.domain, upper)
	}
}

func TestLoadIpset(t *testing.T) {
	for _, c := range []LoadCase{
		LoadCase{
			in:     "ipset=/www.baidu.tech/CN",
			domain: "www.baidu.tech",
			expect: "CN",
		},
		LoadCase{
			in:     "ipset=/mydomain.tech/HK",
			domain: "www.mydomain.tech",
			expect: "HK",
		},
	} {
		p.loadline(c.in)
		ele, err := p.tree.FindDomain(c.domain)
		if err != nil {
			t.Errorf("can not get domain %s value\n", c.domain)
			return
		}

		val := ele.(*policyValue)

		if len(val.ipset) <= 0 {
			t.Error("got empty ipset")
			return
		}

		if val.ipset[0] != c.expect {
			t.Errorf("expect %s got %s\n", c.expect, val.ipset[0])
			return
		}

		t.Logf("domain %s ipset %s\n", c.domain, val.ipset[0])
	}
}

func TestLoadScript(t *testing.T) {
	for _, c := range []LoadCase{
		LoadCase{
			in:     "script=/www.baidu.tech//data/dns/scripts/route.sh",
			domain: "www.baidu.tech",
			expect: "/data/dns/scripts/route.sh",
		},
		LoadCase{
			in:     "script=/mydomain.tech//data/dns/scripts/iptables.sh",
			domain: "www.mydomain.tech",
			expect: "/data/dns/scripts/iptables.sh",
		},
	} {
		p.loadline(c.in)
		ele, err := p.tree.FindDomain(c.domain)
		if err != nil {
			t.Errorf("can not get domain %s value\n", c.domain)
			return
		}

		val := ele.(*policyValue)

		if val.script == "" {
			t.Error("got empty script")
			return
		}

		if val.script != c.expect {
			t.Errorf("expect %s got %s\n", c.expect, val.script)
			return
		}

		t.Logf("domain %s script %s\n", c.domain, val.script)
	}
}

func TestLoadAddress(t *testing.T) {
	for _, c := range []LoadCase{
		LoadCase{
			in:     "address=/www.baidu.tech/47.75.140.107",
			domain: "www.baidu.tech",
			expect: "47.75.140.107",
		},
		LoadCase{
			in:     "address=/mydomain.tech/47.75.140.107",
			domain: "www.mydomain.tech",
			expect: "47.75.140.107",
		},
	} {
		p.loadline(c.in)

		address := p.GetAddress(c.domain)
		if len(address) <= 0 {
			t.Fatalf("got empty,domain=%s\n", c.domain)
			return
		}

		if address[0] != c.expect {
			t.Fatalf("expect %s got %s\n", c.expect, address)
			return
		}

		t.Logf("domain %s address %s\n", c.domain, address)
	}
}
