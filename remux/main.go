package main

import (
	"flag"
	"fmt"
	"log"
	"runtime/debug"

	"github.com/3d0c/gmf"
)

func fatal(err error) {
	debug.PrintStack()
	log.Fatal(err)
}

func assert(i interface{}, err error) interface{} {
	if err != nil {
		fatal(err)
	}

	return i
}

func main() {

	var input, output string

	log.SetFlags(log.Lshortfile | log.Ldate | log.Ltime)

	flag.StringVar(&input, "input", "bbb.mp4", "input file")
	flag.StringVar(&output, "output", "out.aac", "out file")
	flag.Parse()

	inputCtx, err := gmf.NewInputCtx(input)
	defer inputCtx.Free()

	if err != nil {
		panic(err)
	}

	inputCtx.Dump()

	outputCtx, err := gmf.NewOutputCtxWithFormatName(output, "mpegts")
	if err != nil {
		panic(err)
	}
	defer outputCtx.Free()

	for i := 0; i < inputCtx.StreamsCnt(); i++ {
		srcStream, err := inputCtx.GetStream(i)
		if err != nil {
			fmt.Println("GetStream error")
		}
		outStream, err := outputCtx.AddStreamWithCodeCtx(srcStream.CodecCtx())

		par := gmf.NewCodecParameters()
		if err = par.FromContext(srcStream.CodecCtx()); err != nil {
			panic(err)
		}
		defer par.Free()

		outStream.CopyCodecPar(par)
		outStream.SetCodecCtx(srcStream.CodecCtx())
	}
	outputCtx.Dump()

	if err := outputCtx.WriteHeader(); err != nil {
		panic(err)
	}

	for packet := range inputCtx.GetNewPackets() {
		ist, _ := inputCtx.GetStream(packet.StreamIndex())
		ost, _ := outputCtx.GetStream(packet.StreamIndex())

		if packet.Pts() != gmf.AV_NOPTS_VALUE {
			packet.SetPts(gmf.RescaleQRnd(packet.Pts(), ist.TimeBase(), ost.TimeBase()))
		}

		if packet.Dts() != gmf.AV_NOPTS_VALUE {
			packet.SetDts(gmf.RescaleQRnd(packet.Dts(), ist.TimeBase(), ost.TimeBase()))
		}

		if err := outputCtx.WritePacket(packet); err != nil {
			fmt.Println(err)
		}
		packet.Free()
	}

}
