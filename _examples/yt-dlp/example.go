package main

import (
	"bufio"
	"context"
	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/disgo/voice"
	"github.com/disgoorg/json"
	"github.com/disgoorg/log"
	"github.com/disgoorg/snowflake/v2"
	"githubv.com/disgo/ffmpeg-audio"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

var (
	token     = os.Getenv("token")
	guildID   = snowflake.GetEnv("guild_id")
	channelID = snowflake.GetEnv("channel_id")

	commands = []discord.ApplicationCommandCreate{
		discord.SlashCommandCreate{
			Name:        "play",
			Description: "Play a song",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionString{
					Name:        "query",
					Description: "The song to play",
					Required:    true,
				},
			},
		},
	}
)

func main() {
	log.SetLevel(log.LevelInfo)
	log.SetFlags(log.LstdFlags | log.Llongfile)
	log.Info("starting up")

	s := make(chan os.Signal, 1)

	r := handler.New()
	r.Command("/play", onPlay)

	client, err := disgo.New(token,
		bot.WithGatewayConfigOpts(gateway.WithIntents(gateway.IntentGuildVoiceStates)),
		bot.WithEventListeners(r),
	)
	if err != nil {
		log.Fatal("error creating client: ", err)
	}

	if err = handler.SyncCommands(client, commands, []snowflake.ID{guildID}); err != nil {
		log.Fatal("error syncing commands: ", err)
	}

	if err = client.OpenGateway(context.TODO()); err != nil {
		log.Fatal("error connecting to gateway: ", err)
	}
	defer client.Close(context.TODO())

	log.Info("ExampleBot is now running. Press CTRL-C to exit.")
	signal.Notify(s, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-s
}

func onPlay(e *handler.CommandEvent) error {
	query := e.SlashCommandInteractionData().String("query")

	cmd := exec.Command(
		"yt-dlp", query,
		"--extract-audio",
		"--audio-format", "opus",
		"--no-playlist",
		"-o", "-",
		"--quiet",
		"--ignore-errors",
		"--no-warnings",
	)
	cmd.Stderr = os.Stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return e.CreateMessage(discord.MessageCreate{
			Content: "Error creating stdout pipe: " + err.Error(),
		})
	}

	if err = e.DeferCreateMessage(false); err != nil {
		return err
	}

	go func() {
		conn := e.Client().VoiceManager().CreateConn(guildID)
		if err = conn.Open(context.TODO(), channelID, false, false); err != nil {
			println("error connecting to voice channel: ", err)
			e.UpdateInteractionResponse(discord.MessageUpdate{
				Content: json.Ptr("Error connecting to voice channel: " + err.Error()),
			})
		}
		defer conn.Close(context.TODO())

		if err = conn.SetSpeaking(context.TODO(), voice.SpeakingFlagMicrophone); err != nil {
			e.UpdateInteractionResponse(discord.MessageUpdate{
				Content: json.Ptr("Error setting speaking: " + err.Error()),
			})
		}

		if err = cmd.Start(); err != nil {
			e.UpdateInteractionResponse(discord.MessageUpdate{
				Content: json.Ptr("Error starting yt-dlp: " + err.Error()),
			})
		}

		opusProvider, err := ffmpeg.New(bufio.NewReader(stdout))
		if err != nil {
			e.UpdateInteractionResponse(discord.MessageUpdate{
				Content: json.Ptr("Error creating opus provider: " + err.Error()),
			})
		}
		defer opusProvider.Close()

		conn.SetOpusFrameProvider(opusProvider)

		e.UpdateInteractionResponse(discord.MessageUpdate{
			Content: json.Ptr("Playing " + query),
		})

		if err = cmd.Wait(); err != nil {
			log.Error("error waiting for yt-dlp: ", err)
		}
	}()
	return nil
}
