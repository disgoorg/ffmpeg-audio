package ffmpeg

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"

	"github.com/disgoorg/disgo/voice"
	"github.com/jonas747/ogg"
	"golang.org/x/sync/errgroup"
)

const (
	// Exec is the default path to the ffmpeg executable
	Exec       = "ffmpeg"
	Channels   = 2
	SampleRate = 48000
	BufferSize = 65307
)

var _ voice.OpusFrameProvider = (*AudioProvider)(nil)

func NewFFMPEG(ctx context.Context, r io.Reader, opts ...ConfigOpt) (*AudioProvider, error) {
	cfg := DefaultConfig()
	cfg.Apply(opts)

	pr, pw := io.Pipe()
	cmd := exec.CommandContext(ctx, cfg.Exec,
		"-i", "pipe:0",
		"-c:a", "libopus",
		"-ac", strconv.Itoa(cfg.Channels),
		"-ar", strconv.Itoa(cfg.SampleRate),
		"-f", "ogg",
		"-b:a", "96K",
		"pipe:1",
	)
	cmd.Stdin = r
	cmd.Stdout = pw

	done, doneFunc := context.WithCancel(context.Background())
	return &AudioProvider{
		reader: pr,
		writer: pw,

		d:        ogg.NewPacketDecoder(ogg.NewDecoder(bufio.NewReaderSize(pr, cfg.BufferSize))),
		done:     done,
		doneFunc: doneFunc,
		cmd:      cmd,
	}, nil
}

type AudioProvider struct {
	reader *io.PipeReader
	writer *io.PipeWriter

	d        *ogg.PacketDecoder
	done     context.Context
	doneFunc context.CancelFunc

	cmd *exec.Cmd
}

func (p *AudioProvider) ProvideOpusFrame() ([]byte, error) {
	data, _, err := p.d.Decode()
	if err != nil {
		p.doneFunc()

		if errors.Is(err, io.EOF) || errors.Is(err, os.ErrClosed) || errors.Is(err, io.ErrClosedPipe) {
			return nil, io.EOF
		}
		return nil, fmt.Errorf("error decoding ogg packet: %w", err)
	}

	return data, nil
}

func (p *AudioProvider) Close() {
	if err := p.reader.Close(); err != nil {
		// ignore error
	}
	p.doneFunc()
}

func (p *AudioProvider) Wait() error {
	var eg errgroup.Group
	eg.Go(func() error {
		if err := p.cmd.Run(); err != nil {
			return errors.Join(err, p.writer.CloseWithError(err))
		}
		return p.writer.Close()
	})

	eg.Go(func() (err error) {
		<-p.done.Done()
		return
	})

	return eg.Wait()
}
