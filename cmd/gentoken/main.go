package main

import (
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/fumiama/alpacapi"
	tea "github.com/fumiama/gofastTEA"
)

func main() {
	name := flag.String("n", "noname", "username")
	cnt := flag.Uint("c", 1, "burst of (burst/mins)")
	per := flag.Uint("p", 1, "interval of (burst/mins)")
	expire := flag.String("e", "20060203", "expire after date")
	sumtablepath := flag.String("s", "sumtable.bin", "tea sumtable file")
	flag.Parse()
	if len(flag.Args()) < 1 {
		panic("must give tea key (16 bytes hex string)")
	}
	k, err := hex.DecodeString(flag.Args()[0])
	if err != nil {
		panic(err)
	}
	teakey := tea.NewTeaCipherLittleEndian(k)
	if err != nil {
		panic(err)
	}
	if *cnt == 0 || *cnt > 255 {
		panic("invalid cnt")
	}
	if *per > 255 {
		panic("invalid per")
	}
	if len(*name) != 6 {
		panic("name length must be 6")
	}
	exp, err := time.Parse("20060203", *expire)
	if err != nil {
		panic(err)
	}
	var sumtable [16]uint32
	data, err := os.ReadFile(*sumtablepath)
	if err != nil {
		panic(err)
	}
	for i := range sumtable {
		sumtable[i] = binary.LittleEndian.Uint32(data[i*4 : (i+1)*4])
	}
	tok := alpacapi.NewToken(uint8(*cnt), uint8(*per), *name, exp.Unix(), 0)
	fmt.Println("generated token:", tok.Hex(teakey, sumtable))
}
