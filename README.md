### dns proxy by golang, if get any bug, let me know
1. Compatible with dnsmasq configuration file (including ipset related configuration)

	server=/baidu.com/8.8.8.8#53  
	ipset=/whatsapp.com/US-DNS,US-DNSv6  
	address=/baidu.com/192.168.100.100  
	script=/baidu.com//etc/dnsproxy/route.sh

2. radix trie, 基于他人的基础上增加一些域名查找的接口 [go-radix](https://github.com/jursonmo/go-radix)
