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

	if err := Validate(cfg); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	return
}

func Validate(cfg *Cfg) error {
	if cfg == nil {
		return fmt.Errorf("config is required")
	}
	if cfg.Connect == "" && cfg.Serve == "" {
		return fmt.Errorf("you should specify --conn or --serve")
	}
	if cfg.TunType == "" {
		return fmt.Errorf("you should specify --tun")
	}
	return nil
}
