package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/snowflake/v2"

	"github.com/disgoorg/ffmpeg-audio"
)

var (
	token     = os.Getenv("token")
	guildID   = snowflake.GetEnv("guild_id")
	channelID = snowflake.GetEnv("channel_id")
)

func main() {
	slog.SetLogLoggerLevel(slog.LevelDebug)
	slog.Info("starting up")

	client, err := disgo.New(token,
		bot.WithGatewayConfigOpts(gateway.WithIntents(gateway.IntentGuildVoiceStates)),
		bot.WithEventListenerFunc(func(e *events.Ready) {
			go play(e.Client())
		}),
	)
	if err != nil {
		slog.Error("error creating client", slog.Any("err", err))
		return
	}

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		client.Close(ctx)
	}()

	if err = client.OpenGateway(context.TODO()); err != nil {
		slog.Error("error connecting to gateway", slog.Any("err", err))
		return
	}

	slog.Info("ExampleBot is now running. Press CTRL-C to exit.")
	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-s
}

func play(client *bot.Client) {
	for {
		if err := PlaySound(context.Background(), client, guildID, channelID, "https://cdn.discordapp.com/soundboard-sounds/1394685689790206032"); err != nil {
			slog.Error("error playing sound", slog.Any("err", err))
		}
		time.Sleep(2 * time.Second)
	}
}

func PlaySound(ctx context.Context, client *bot.Client, guildID, channelID snowflake.ID, url string) error {
	conn := client.VoiceManager.CreateConn(guildID)

	if err := conn.Open(ctx, channelID, false, true); err != nil {
		return fmt.Errorf("error connecting to voice channel: %w", err)
	}

	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		conn.Close(cleanupCtx)
	}()

	rs, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("error opening sound URL: %w", err)
	}
	defer rs.Body.Close()

	// Stream through ffmpeg to get Opus frames
	opusProvider := ffmpeg.New(ctx, rs.Body, ffmpeg.WithExec("ffmpeg"))

	conn.SetOpusFrameProvider(opusProvider)

	if err := opusProvider.Wait(); err != nil {
		fmt.Printf("opus provider wait error: %T: %v\n", err, err)
		return fmt.Errorf("error waiting for opus provider: %w", err)
	}

	return nil
}
