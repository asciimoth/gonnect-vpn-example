package cfg

import (
	"flag"
	"fmt"
	"os"
)

type Cfg struct {
	// Transport config
	Connect string
	Serve   string

	// Tun config
	TunType      string
	TunName      string
	TunAddr      string
	TunSubnet    string
	TunHttpAddr  string
	TunSocksAddr string
}

func Load() (cfg *Cfg) {
	cfg = &Cfg{}

	flag.StringVar(&cfg.Connect, "conn", "", "ws server address")
	flag.StringVar(&cfg.Serve, "serve", "", "address to start ws server")
	flag.StringVar(&cfg.TunType, "tun", "", "tun device type (vtun+http | vtun+socks | native)")
	flag.StringVar(&cfg.TunName, "name", "", "tun device name")
	flag.StringVar(&cfg.TunAddr, "addr", "", "tun address")
	flag.StringVar(&cfg.TunSubnet, "subnet", "", "tun subnet")
	flag.StringVar(&cfg.TunHttpAddr, "http", "", "listening addr for vtun+http")
	flag.StringVar(&cfg.TunSocksAddr, "socks", "", "local socks server addr for socks+http")
	flag.Parse()

	if cfg.Connect == "" && cfg.Serve == "" {
		fmt.Println("You should specify --conn or --serve")
		os.Exit(1)
	}
	if cfg.TunType == "" {
		fmt.Println("You should specify --tun")
		os.Exit(1)
	}
	return
}
