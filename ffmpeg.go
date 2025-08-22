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
	"sync"

	"github.com/disgoorg/disgo/voice"
	"github.com/jonas747/ogg"
)

const (
	// Exec is the default path to the ffmpeg executable
	Exec       = "ffmpeg"
	Channels   = 2
	SampleRate = 48000
	BufferSize = 65307
)

var _ voice.OpusFrameProvider = (*AudioProvider)(nil)

func New(ctx context.Context, r io.Reader, opts ...ConfigOpt) (*AudioProvider, error) {
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

	go func() {
		if err := cmd.Run(); err != nil {
			pw.CloseWithError(err)
			return
		}
		pw.Close()
	}()

	done, doneFunc := context.WithCancel(context.Background())
	return &AudioProvider{
		source:   pr,
		d:        ogg.NewPacketDecoder(ogg.NewDecoder(bufio.NewReaderSize(pr, cfg.BufferSize))),
		done:     done,
		doneFunc: doneFunc,
	}, nil
}

type AudioProvider struct {
	source   io.Reader
	d        *ogg.PacketDecoder
	done     context.Context
	doneFunc context.CancelFunc
}

func (p *AudioProvider) ProvideOpusFrame() ([]byte, error) {
	data, _, err := p.d.Decode()
	if err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, os.ErrClosed) || errors.Is(err, io.ErrClosedPipe) {
			p.doneFunc()
			return nil, io.EOF
		}
		return nil, fmt.Errorf("error decoding ogg packet: %w", err)
	}

	return data, nil
}

func (p *AudioProvider) Close() {
	if c, ok := p.source.(io.Closer); ok {
		_ = c.Close()
	}
	p.doneFunc()
}

func (p *AudioProvider) Wait() error {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-p.done.Done()
	}()

	wg.Wait()
	return nil
}
