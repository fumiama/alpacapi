package main

import (
	"encoding/binary"
	"encoding/hex"
	"flag"
	"net"
	"net/http"
	"net/netip"
	"os"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/fumiama/alpacapi"
	para "github.com/fumiama/go-hide-param"
	tea "github.com/fumiama/gofastTEA"
)

var (
	buffersize uint
	teakey     tea.TEA
	sumtable   [16]uint32
	workers    []*net.UDPAddr
	timeout    time.Duration
)

func main() {
	addr := flag.String("l", "0.0.0.0:31471", "listening endpoint")
	bufsz := flag.Uint("b", 4096, "udp buffer size")
	sumtablepath := flag.String("s", "sumtable.bin", "tea sumtable file")
	to := flag.Uint("t", 5, "timeout (mins)")
	flag.Parse()
	if len(flag.Args()) < 2 {
		panic("must give tea key (16 bytes hex string) and worker endpoints")
	}
	buffersize = *bufsz
	timeout = time.Minute * time.Duration(*to)
	k, err := hex.DecodeString(flag.Args()[0])
	if err != nil {
		panic(err)
	}
	para.Hide(1)
	teakey = tea.NewTeaCipherLittleEndian(k)
	data, err := os.ReadFile(*sumtablepath)
	if err != nil {
		panic(err)
	}
	for i := range sumtable {
		sumtable[i] = binary.LittleEndian.Uint32(data[i*4 : (i+1)*4])
	}
	workers = make([]*net.UDPAddr, len(flag.Args()[1:]))
	for i, ep := range flag.Args()[1:] {
		workers[i] = net.UDPAddrFromAddrPort(netip.MustParseAddrPort(ep))
		logrus.Infoln("add worker:", workers[i])
	}
	http.HandleFunc("/reply", reply)
	logrus.Infoln("listening on", *addr)
	tok := alpacapi.NewToken(1, 0, "fumiam", time.Now().Add(time.Hour).Unix(), 0)
	logrus.Infoln("test token:", tok.Hex(teakey, sumtable))
	logrus.Fatal(http.ListenAndServe(*addr, nil))
}
