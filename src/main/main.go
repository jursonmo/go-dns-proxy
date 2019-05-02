package main

import (
	"dnsproxy"
	"flag"
	"fmt"
)

var (
	version = "1.0.0"
)

func main() {
	flgConf := flag.String("c", "../config/config.toml", "config file")
	flgVersion := flag.Bool("v", false, "print version")
	flag.Parse()

	if *flgVersion {
		fmt.Println(version)
		return
	}

	dnsproxy.Main(*flgConf)
}
