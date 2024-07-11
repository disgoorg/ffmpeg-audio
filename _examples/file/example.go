package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/disgo/voice"
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

	s := make(chan os.Signal, 1)

	client, err := disgo.New(token,
		bot.WithGatewayConfigOpts(gateway.WithIntents(gateway.IntentGuildVoiceStates)),
		bot.WithEventListenerFunc(func(e *events.Ready) {
			go play(e.Client(), s)
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
	signal.Notify(s, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-s
}

func play(client bot.Client, closeChan chan os.Signal) {
	conn := client.VoiceManager().CreateConn(guildID)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	if err := conn.Open(ctx, channelID, false, false); err != nil {
		panic("error connecting to voice channel: " + err.Error())
	}
	defer func() {
		closeCtx, closeCancel := context.WithTimeout(context.Background(), time.Second*10)
		defer closeCancel()
		conn.Close(closeCtx)
	}()

	if err := conn.SetSpeaking(ctx, voice.SpeakingFlagMicrophone); err != nil {
		panic("error setting speaking flag: " + err.Error())
	}

	defer func() {
		closeChan <- syscall.SIGTERM
	}()

	for {
		func() {
			file, err := os.Open("test.mp3")
			if err != nil {
				panic("error opening file: " + err.Error())
			}

			opusProvider, err := ffmpeg.New(context.Background(), file)
			if err != nil {
				panic("error creating opus provider: " + err.Error())
			}
			defer opusProvider.Close()

			conn.SetOpusFrameProvider(opusProvider)
			if err = opusProvider.Wait(); err != nil {
				panic("error waiting for opus provider: " + err.Error())
			}
		}()
	}

}
