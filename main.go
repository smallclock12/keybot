package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"sync/atomic"

	"fmt"

	"github.com/bwmarrin/discordgo"
)

const (
	STARTING = 1 << iota
	READY
)


var keyCheckCommand *discordgo.ApplicationCommand = &discordgo.ApplicationCommand{
	Name: "check-key",
	Description: "Check your key!",
	Options: []*discordgo.ApplicationCommandOption{{
		Type:                     discordgo.ApplicationCommandOptionString,
		Name:                     "keyname",
		Description:              "[<item>:]<key_name> example: redstone_dust:1@k-446-ske-20659-schemes",
		Required:                 true,
	}},
}

var cooldownTracker map[string]time.Time = map[string]time.Time{}

func main() {
	var status atomic.Int32
	status.Swap(STARTING)

	key := strings.Split(os.Getenv("SMALLCLOCK12_KEY"), "-")
	item := os.Getenv("SMALLCLOCK12_ITEM")
	token := os.Getenv("SMALLCLOCK12_TOKEN")
	user := os.Getenv("SMALLCLOCK12_USER")
	cooldown, _ := strconv.Atoi(os.Getenv("SMALLCLOCK12_COOLDOWN"))
	webhook := os.Getenv("SMALLCLOCK12_WEBHOOK")
	webhookName := os.Getenv("SMALLCLOCK12_WEBHOOK_NAME")

	disc, err := discordgo.New("Bot " + token)
	if err != nil {
		panic(err)
	}

	disc.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		status.Swap(READY)
		log.Printf("Session ready")
	}) 

	disc.AddHandler(func(s *discordgo.Session, r *discordgo.InteractionCreate) {
		userId := ""
		if r.User != nil {
			userId = r.User.ID
		} else if r.Member != nil && r.Member.User != nil && r.Member.User.ID == user {
			userId = r.Member.User.ID
		}

		if userId == "" {
			respondCommand("Please add me as an application & DM me for a response!", s, r)
			return
		}

		if d := r.ApplicationCommandData(); d.Name == keyCheckCommand.Name {
			g := d.Options[0].StringValue()
			message := fmt.Sprintf("Command interaction! User: %s, Checking: %s", userId, g)
			go sendToWebhook(webhook, webhookName, message)
			log.Printf(message)
			n := time.Now()
			x := cooldownTracker[userId]
			if n.Before(x) {
				respondCommand(fmt.Sprintf("You are on cooldown! You can try again <t:%d:R>", x.Unix()), s, r)
				return
			}

			res := compareParts(key, item, g)
			if res == -1 {
				respondCommand("Could not process key!", s, r)
			} else {
				respondCommand(fmt.Sprint(res), s, r)
				if userId != user {
					cooldownTracker[userId] = n.Add(time.Minute*time.Duration(cooldown))
				}
			}
		}
	}) 


	err = disc.Open()
	if err != nil {
		panic(err)
	}
	defer disc.Close()	

	_, err = disc.ApplicationCommandCreate(disc.State.User.ID, "", keyCheckCommand);
	if err != nil {
		panic(err)
	}

	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, os.Interrupt)
	<-sigch
}

func respondCommand(message string, s *discordgo.Session, r *discordgo.InteractionCreate) {
	s.InteractionRespond(r.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
		},
	})

}

func sendToWebhook(webhook string, appName string, content string) {
	if webhook == "" {
		return
	}

	body := fmt.Sprintf("{\"content\": \"%s\", \"username\":\"%s\"}", content, appName)
	_, err := http.Post(webhook, "application/json", strings.NewReader(body))
	if err != nil {
		log.Print(err)
	}
}

func compareParts(key []string, item string, guess string) int {

	c := 0
	split := strings.Split(guess, ":")
	if len(split) == 0 || len(split) > 2 {
		return -1
	}

	itemGuess := ""
	keyGuess := split[0]
	if len(split) > 1 {
		keyGuess = split[1]
		itemGuess = split[0]
	}

	if itemGuess != "" {
		if itemGuess == item {
			c++
		}
	}

	if keyGuess != "" {
		guessSplit := strings.Split(keyGuess, "-")
		if len(guessSplit) == len(key) {
			for i := range guessSplit {
				if guessSplit[i] == key[i] {
					c++
				}
			}
		}
	}

	return c
}
