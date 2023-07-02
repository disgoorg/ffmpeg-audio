package ffmpeg

// DefaultConfig returns a new Config with default values.
func DefaultConfig() *Config {
	return &Config{
		Exec:       Exec,
		SampleRate: SampleRate,
		Channels:   Channels,
		BufferSize: BufferSize,
	}
}

// Config is used to configure a ffmpeg audio source.
type Config struct {
	Exec       string
	SampleRate int
	Channels   int
	BufferSize int
}

// ConfigOpt is used to functionally configure a Config.
type ConfigOpt func(config *Config)

// Apply applies the ConfigOpt(s) to the Config.
func (c *Config) Apply(opts []ConfigOpt) {
	for _, opt := range opts {
		opt(c)
	}
}

// WithExec sets the Config(s) used Exec.
func WithExec(exec string) ConfigOpt {
	return func(config *Config) {
		config.Exec = exec
	}
}

// WithSampleRate sets the Config(s) used SampleRate.
func WithSampleRate(sampleRate int) ConfigOpt {
	return func(config *Config) {
		config.SampleRate = sampleRate
	}
}

// WithChannels sets the Config(s) used Channels.
func WithChannels(channels int) ConfigOpt {
	return func(config *Config) {
		config.Channels = channels
	}
}

// WithBufferSize sets the Config(s) used BufferSize.
func WithBufferSize(bufferSize int) ConfigOpt {
	return func(config *Config) {
		config.BufferSize = bufferSize
	}
}
