package main

import (
	"flag"
	"io"
	"log"

	"github.com/3d0c/gmf"
)

var (
	input  string
	output string
)

func main() {

	flag.StringVar(&input, "input", "bbb.mp4", "input file")
	flag.StringVar(&output, "output", "out.aac", "out file")
	flag.Parse()

	log.SetFlags(log.Lshortfile)

	var (
		ictx, octx *gmf.FmtCtx
		ist, ost   *gmf.Stream
		pkt        *gmf.Packet
		op         []*gmf.Packet
		codec      *gmf.Codec
		cc         *gmf.CodecCtx
		frames     []*gmf.Frame
		err        error
	)

	if ictx, err = gmf.NewInputCtx(input); err != nil {
		log.Fatalf("Error creating context - %s\n", err)
	}
	defer ictx.Free()

	if octx, err = gmf.NewOutputCtx(output); err != nil {
		log.Fatalln(err)
	}
	defer octx.Free()

	codec, err = gmf.FindEncoder("aac")
	if err != nil {
		log.Fatalf("Error finding codec - %s\n", err)
	}

	if cc = gmf.NewCodecCtx(codec); cc == nil {
		log.Fatalf("unable to create codec context")
	}

	cc.SetSampleFmt(cc.SelectSampleFmt())
	cc.SetSampleRate(48000)
	cc.SetChannels(2)
	channelLayout := cc.SelectChannelLayout()
	cc.SetChannelLayout(channelLayout)
	cc.SetTimeBase(gmf.AVR{Num: 1, Den: 48000})
	defer cc.Free()

	if err := cc.Open(nil); err != nil {
		log.Fatalln(err)
	}

	if ost, err = octx.AddStreamWithCodeCtx(cc); err != nil {
		log.Fatalln(err)
	}
	defer ost.Free()

	ost.SetCodecCtx(cc)

	octx.WriteHeader()

	for {
		if pkt, err = ictx.GetNextPacket(); err != nil {
			if err == io.EOF {
				break
			} else {
				log.Fatalln(pkt, err)
			}
		}

		ist, err = ictx.GetStream(pkt.StreamIndex())
		if err != nil {
			log.Fatalln(err)
		}

		if ist.Type() != gmf.AVMEDIA_TYPE_AUDIO {
			pkt.Free()
			continue
		}

		if ost.SwrCtx == nil {
			icc := ist.CodecCtx()
			occ := ost.CodecCtx()
			options := []*gmf.Option{
				{Key: "in_channel_layout", Val: icc.ChannelLayout()},
				{Key: "out_channel_layout", Val: occ.ChannelLayout()},
				{Key: "in_sample_rate", Val: icc.SampleRate()},
				{Key: "out_sample_rate", Val: occ.SampleRate()},
				{Key: "in_sample_fmt", Val: gmf.SampleFormat(icc.SampleFmt())},
				{Key: "out_sample_fmt", Val: gmf.SampleFormat(occ.SampleFmt())},
			}

			if ost.SwrCtx, err = gmf.NewSwrCtx(options, occ.Channels(), occ.SampleFmt()); err != nil {
				panic(err)
			}
			ost.AvFifo = gmf.NewAVAudioFifo(icc.SampleFmt(), ist.CodecCtx().Channels(), 1024)
		}

		if frames, err = ist.CodecCtx().Decode(pkt); err != nil {
			log.Fatalln(err)
		}

		frames = gmf.DefaultResampler(ost, frames, false)

		if op, err = ost.CodecCtx().Encode(frames, -1); err != nil {
			log.Fatalln(err)
		}

		for i, _ := range frames {
			frames[i].Free()
		}

		for i, _ := range op {
			octx.WritePacketNoBuffer(op[i])
			op[i].Free()
		}

		pkt.Free()

	}

	octx.WriteTrailer()

	for i := 0; i < ictx.StreamsCnt(); i++ {
		st, _ := ictx.GetStream(i)
		st.CodecCtx().Free()
		st.Free()
	}

}
