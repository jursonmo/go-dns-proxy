package dnsproxy

import (
	"fmt"
	"net"
	"strings"
	"time"

	logs "github.com/jursonmo/beelogs"
	"github.com/miekg/dns"
)

var (
	defaultAddr        = ":53"
	defaultTimeout     = 5
	defaultConcurrency = 10
	defaultQueueSize   = defaultConcurrency * 5
)

type ProxyConfig struct {
	Upper       []string `toml:"upper"`
	ListenAddr  string   `toml:"listen_addr"`
	Timeout     int      `toml:"timeout"`
	Concurrency int      `toml:"concurrency"`
	QueueSize   int      `toml:"queue_size"`
}

type clientContext struct {
	conn  *net.UDPConn
	raddr *net.UDPAddr
	buf   []byte
}

type Proxy struct {
	listenAddr  string
	upper       []string
	concurrency int
	qsize       int
	timeout     time.Duration

	done   chan struct{}
	cache  *Cache
	policy *Policy
	queue  chan *clientContext
}

func NewProxy(cfg *ProxyConfig, cache *Cache, policy *Policy) *Proxy {
	if len(cfg.Upper) <= 0 {
		logs.Error("dns.upper MUST NOT be empty")
		return nil
	}

	listenAddr := cfg.ListenAddr
	if listenAddr == "" {
		listenAddr = defaultAddr
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	concurrency := cfg.Concurrency
	if concurrency <= 0 {
		concurrency = defaultConcurrency
	}

	qsize := cfg.QueueSize
	if qsize <= 0 {
		qsize = defaultQueueSize
	}

	return &Proxy{
		listenAddr:  listenAddr,
		upper:       cfg.Upper,
		concurrency: concurrency,
		qsize:       qsize,
		timeout:     time.Duration(timeout) * time.Second,
		done:        make(chan struct{}),
		cache:       cache,
		policy:      policy,
		queue:       make(chan *clientContext, qsize),
	}
}

func (p *Proxy) Run() error {
	laddr, err := net.ResolveUDPAddr("udp", p.listenAddr)
	if err != nil {
		return err
	}

	conn, err := net.ListenUDP("udp", laddr)
	if err != nil {
		return err
	}
	defer conn.Close()

	for i := 0; i < p.concurrency; i++ {
		go p.handleQuery()
	}

	logs.Info("dns running on %s", p.listenAddr)

	for {
		select {
		case <-p.done:
			return nil
		default:
		}

		buf := make([]byte, 512)
		nr, raddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			return err
		}
		p.onQuery(conn, raddr, buf[:nr])
	}

}

func (p *Proxy) Stop() {
	close(p.done)
}

func (p *Proxy) onQuery(conn *net.UDPConn, raddr *net.UDPAddr, buf []byte) {
	// go p.handleQuery(conn, raddr, buf)
	p.queue <- &clientContext{conn, raddr, buf}
}

func (p *Proxy) handleQuery() {
	for {
		select {
		case <-p.done:
			logs.Warn("proxy recv done signal")
			return

		case ctx := <-p.queue:
			conn, raddr, buf := ctx.conn, ctx.raddr, ctx.buf

			req := &dns.Msg{}
			err := req.Unpack(buf)
			if err != nil {
				logs.Warn("invalid dns request: %v", err)
				continue
			}

			if len(req.Question) <= 0 {
				logs.Warn("empty question")
				continue
			}

			domain := strings.ToLower(req.Question[0].Name)
			//请求域名后面有加"."
			//fmt.Printf("debug, domain:%s\n", domain)// "baidu.com."
			if domain[len(domain)-1] == '.' {
				domain = domain[:len(domain)-1]
			}
			address := p.policy.GetAddress(domain)
			if len(address) > 0 {
				err = p.handleAddress(domain, conn, raddr, req, address)
				if err == nil {
					logs.Debug("%s => %s", domain, "buildin")
					continue
				}
			}

			if p.cache != nil {
				// 缓存查询仅针对A记录和AAAA记录
				support := true
				for _, as := range req.Question {
					if as.Qtype != dns.TypeA && as.Qtype != dns.TypeAAAA {
						support = false
					}
				}

				if support {
					ele := p.cache.Get(domain)
					if ele != nil {
						err = p.handleCache(domain, conn, raddr, req, ele.(*dns.Msg))
						if err == nil {
							logs.Debug("%s => %s", domain, "cache")
							continue
						}
					}
				}
			}

			upper := p.upper
			if p.policy != nil {
				pupper := p.policy.GetUpper(domain)
				if len(pupper) > 0 {
					upper = pupper
				}
			}

			for _, up := range upper {
				resp, err := p.resolve(up, buf)
				if err != nil {
					logs.Warn("resolve from upper: %s fail: %v", up, err)
					continue
				}

				err = p.handleResult(domain, conn, raddr, resp)
				if err != nil {
					logs.Warn("response result fail: %v", err)
					continue
				}

				logs.Debug("%s => %s", domain, up)
				if p.cache != nil {
					// 缓存存储仅针对A记录和AAAA记录
					needcache := true
					for _, as := range resp.Answer {
						hdr := as.Header()
						if hdr.Rrtype != dns.TypeA && hdr.Rrtype != dns.TypeAAAA {
							needcache = false
							break
						}
					}

					if needcache {
						p.cache.Set(domain, resp)
					}
				}

				break
			}
		}
	}
}

func (p *Proxy) resolve(upper string, buf []byte) (*dns.Msg, error) {
	conn, err := net.DialTimeout("udp", upper, p.timeout)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	conn.SetWriteDeadline(time.Now().Add(p.timeout))
	_, err = conn.Write(buf)
	conn.SetWriteDeadline(time.Time{})
	if err != nil {
		return nil, err
	}

	res := make([]byte, 512)
	conn.SetReadDeadline(time.Now().Add(p.timeout))
	nr, err := conn.Read(res)
	conn.SetWriteDeadline(time.Time{})
	if err != nil {
		return nil, err
	}

	rmsg := &dns.Msg{}
	err = rmsg.Unpack(res[:nr])
	if err != nil {
		return nil, err
	}

	return rmsg, nil
}

func (p *Proxy) handleAddress(domain string, conn *net.UDPConn, raddr *net.UDPAddr, req *dns.Msg, address []string) error {
	resp := req.Copy()
	resp.Response = true

	for _, q := range req.Question {
		for _, ip := range address {
			hdr := dns.RR_Header{Name: q.Name, Class: q.Qclass, Ttl: 60}

			addr := net.ParseIP(ip)
			if addr.To4() != nil {
				hdr.Rrtype = dns.TypeA
				hdr.Rdlength = 4
				resp.Answer = append(resp.Answer, &dns.A{Hdr: hdr, A: addr})
			} else if addr.To16() != nil {
				hdr.Rrtype = dns.TypeAAAA
				hdr.Rdlength = 16
				resp.Answer = append(resp.Answer, &dns.AAAA{Hdr: hdr, AAAA: addr})
			}

		}
	}

	return p.handleResult(domain, conn, raddr, resp)
}

func (p *Proxy) handleCache(domain string, conn *net.UDPConn, raddr *net.UDPAddr, req, cv *dns.Msg) error {
	resp := req.Copy()
	resp.Response = true

	for _, qs := range req.Question {
		for _, as := range cv.Answer {
			hdr := *as.Header()

			if qs.Qtype == hdr.Rrtype {
				switch hdr.Rrtype {
				case dns.TypeA:
					if an, ok := as.(*dns.A); ok && an != nil {
						resp.Answer = append(resp.Answer, &dns.A{Hdr: hdr, A: an.A})
					}

				case dns.TypeAAAA:
					if an, ok := as.(*dns.AAAA); ok && an != nil {
						resp.Answer = append(resp.Answer, &dns.AAAA{Hdr: hdr, AAAA: an.AAAA})
					}

				default:
					return fmt.Errorf("unsupported")

				}
			}
		}
	}

	if len(resp.Answer) <= 0 {
		return fmt.Errorf("empty answer")
	}

	return p.handleResult(domain, conn, raddr, resp)
}

func (p *Proxy) handleResult(domain string, conn *net.UDPConn, raddr *net.UDPAddr, res *dns.Msg) error {
	if p.policy != nil {
		p.policy.Exec(domain, res)
	}

	msg, err := res.Pack()
	if err != nil {
		return err
	}

	conn.SetWriteDeadline(time.Now().Add(p.timeout))
	_, err = conn.WriteToUDP(msg, raddr)
	conn.SetWriteDeadline(time.Time{})
	if err != nil {
		return err
	}

	return nil
}
