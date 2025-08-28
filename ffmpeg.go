package ffmpeg

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
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

func New(ctx context.Context, r io.Reader, opts ...ConfigOpt) *AudioProvider {
	cfg := DefaultConfig()
	cfg.Apply(opts)

	cmdCtx, cancel := context.WithCancel(ctx)

	pr, pw := io.Pipe()
	cmd := exec.CommandContext(cmdCtx, cfg.Exec,
		"-i", "pipe:0",
		"-c:a", "libopus",
		"-ac", strconv.Itoa(cfg.Channels),
		"-ar", strconv.Itoa(cfg.SampleRate),
		"-f", "ogg",
		"-b:a",
		"96K",
		"pipe:1",
	)
	cmd.Stdin = r

	done := make(chan error)
	var once sync.Once
	doneFunc := func(err error) {
		once.Do(func() {
			done <- err
			close(done)
		})
	}

	go func() {
		err := cmd.Run()
		_ = pw.CloseWithError(err)
	}()

	return &AudioProvider{
		cmd:       cmd,
		cmdCancel: cancel,
		pr:        pr,
		done:      done,
		doneFunc:  doneFunc,
		decoder:   ogg.NewPacketDecoder(ogg.NewDecoder(bufio.NewReaderSize(pr, cfg.BufferSize))),
	}
}

type AudioProvider struct {
	cmd       *exec.Cmd
	cmdCancel context.CancelFunc
	pr        *io.PipeReader
	done      <-chan error
	doneFunc  func(error)
	decoder   *ogg.PacketDecoder
}

func (p *AudioProvider) ProvideOpusFrame() ([]byte, error) {
	data, _, err := p.decoder.Decode()
	if err != nil {
		if errors.Is(err, io.EOF) {
			p.doneFunc(nil)
			return nil, err
		}
		p.doneFunc(err)
		return nil, fmt.Errorf("error decoding ogg packet: %w", err)
	}

	return data, nil
}

func (p *AudioProvider) Close() {
	p.cmdCancel()
}

func (p *AudioProvider) Wait() error {
	err := <-p.done
	_ = p.pr.CloseWithError(err)
	return err
}
