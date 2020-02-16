package main

import (
	"flag"
	"fmt"
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
	flag.Parse()

	var (
		ictx, octx *gmf.FmtCtx
		iast, ivst *gmf.Stream
		oast, ovst *gmf.Stream
		ist        *gmf.Stream
		pkt        *gmf.Packet
		audioCodec *gmf.Codec
		audioCc    *gmf.CodecCtx
		err        error
	)

	if ictx, err = gmf.NewInputCtx(input); err != nil {
		log.Fatalf("Error creating context - %s\n", err)
	}
	defer ictx.Free()

	// if octx, err = gmf.NewOutputCtx(output); err != nil {
	// 	log.Fatalln(err)
	// }
	// defer octx.Free()

	octx = gmf.NewCtx()
	if octx == nil {
		panic("ortx is null")
	}

	iast, err = ictx.GetBestStream(gmf.AVMEDIA_TYPE_AUDIO)
	if err != nil {
		panic(err)
	}
	fmt.Println(iast)

	// audio
	audioCodec, err = gmf.FindEncoder("libopus")
	if err != nil {
		log.Fatalf("Error finding codec - %s\n", err)
	}

	if audioCc = gmf.NewCodecCtx(audioCodec); audioCc == nil {
		log.Fatalf("unable to create codec context")
	}

	audioCc.SetSampleFmt(audioCc.SelectSampleFmt())
	audioCc.SetSampleRate(48000)
	audioCc.SetChannels(2)
	channelLayout := audioCc.SelectChannelLayout()
	audioCc.SetChannelLayout(channelLayout)
	audioCc.SetTimeBase(gmf.AVR{Num: 1, Den: 48000})
	defer audioCc.Free()

	if err := audioCc.Open(nil); err != nil {
		log.Fatalln(err)
	}

	par := gmf.NewCodecParameters()
	if err = par.FromContext(audioCc); err != nil {
		panic(err)
	}
	defer par.Free()

	if oast, err = octx.AddStreamWithCodeCtx(audioCc); oast == nil {
		panic(err)
	}

	oast.SetCodecCtx(audioCc)

	// video
	ivst, err = ictx.GetBestStream(gmf.AVMEDIA_TYPE_VIDEO)
	if err != nil {
		panic(err)
	}

	par = gmf.NewCodecParameters()
	if err = par.FromContext(ivst.CodecCtx()); err != nil {
		panic(err)
	}
	defer par.Free()

	ovst, err = octx.AddStreamWithCodeCtx(ivst.CodecCtx())
	if err != nil {
		panic(err)
	}

	ovst.CopyCodecPar(par)
	ovst.SetCodecCtx(ivst.CodecCtx())

	if octx.IsGlobalHeader() {
		ovst.CodecCtx().SetFlag(gmf.CODEC_FLAG_GLOBAL_HEADER)
	}

	fmt.Println(ovst)

	//octx.WriteHeader()

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

		if ist.Type() == gmf.AVMEDIA_TYPE_AUDIO {
			if oast.SwrCtx == nil {
				icc := ist.CodecCtx()
				occ := oast.CodecCtx()
				options := []*gmf.Option{
					{Key: "in_channel_layout", Val: icc.ChannelLayout()},
					{Key: "out_channel_layout", Val: occ.ChannelLayout()},
					{Key: "in_sample_rate", Val: icc.SampleRate()},
					{Key: "out_sample_rate", Val: occ.SampleRate()},
					{Key: "in_sample_fmt", Val: gmf.SampleFormat(icc.SampleFmt())},
					{Key: "out_sample_fmt", Val: gmf.SampleFormat(occ.SampleFmt())},
				}

				if oast.SwrCtx, err = gmf.NewSwrCtx(options, occ.Channels(), occ.SampleFmt()); err != nil {
					panic(err)
				}
				oast.AvFifo = gmf.NewAVAudioFifo(icc.SampleFmt(), ist.CodecCtx().Channels(), 1024)
			}

			frames, err := ist.CodecCtx().Decode(pkt)
			if err != nil {
				log.Fatalln(err)
			}
			frames = gmf.DefaultResampler(oast, frames, false)
			for _, frame := range frames {
				frame.SetPts(pkt.Pts())
			}
			var packets []*gmf.Packet
			if packets, err = oast.CodecCtx().Encode(frames, -1); err != nil {
				log.Fatalln(err)
			}
			for i, _ := range frames {
				frames[i].Free()
			}
			for i, _ := range packets {

				// if pkt.Pts() != gmf.AV_NOPTS_VALUE {
				// 	pkt.SetPts(gmf.RescaleQRnd(pkt.Pts(), iast.TimeBase(), oast.TimeBase()))
				// }

				// if pkt.Dts() != gmf.AV_NOPTS_VALUE {
				// 	pkt.SetDts(gmf.RescaleQRnd(pkt.Dts(), iast.TimeBase(), oast.TimeBase()))
				// }
				gmf.RescaleTs(packets[i], iast.TimeBase(), oast.TimeBase())
				packets[i].SetStreamIndex(oast.Index())
				//octx.WritePacket(packets[i])
				fmt.Println("audio", packets[i].Pts())
				packets[i].Free()

			}
		} else if ist.Type() == gmf.AVMEDIA_TYPE_VIDEO {
			if pkt.Pts() != gmf.AV_NOPTS_VALUE {
				pkt.SetPts(gmf.RescaleQRnd(pkt.Pts(), ivst.TimeBase(), ovst.TimeBase()))
			}
			if pkt.Dts() != gmf.AV_NOPTS_VALUE {
				pkt.SetDts(gmf.RescaleQRnd(pkt.Dts(), ivst.TimeBase(), ovst.TimeBase()))
			}
			pkt.SetStreamIndex(ovst.Index())

			//octx.WritePacket(pkt)
			fmt.Println("video", pkt.Pts())
		}
		pkt.Free()
	}

	//octx.WriteTrailer()
}
