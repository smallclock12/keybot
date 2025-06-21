package main

import (
	"bytes"
	"encoding/json"
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

type interaction struct {
	userId string
	username string
	command string
	content string
	result string
	timestamp time.Time
}

var cooldownTracker map[string]time.Time = map[string]time.Time{}

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

var key = strings.Split(os.Getenv("SMALLCLOCK12_KEY"), "-")
var item = os.Getenv("SMALLCLOCK12_ITEM")
var token = os.Getenv("SMALLCLOCK12_TOKEN")
var owner = os.Getenv("SMALLCLOCK12_USER")
var cooldown, _ = strconv.Atoi(os.Getenv("SMALLCLOCK12_COOLDOWN"))
var webhook = os.Getenv("SMALLCLOCK12_WEBHOOK")
var webhookName = os.Getenv("SMALLCLOCK12_WEBHOOK_NAME")

func main() {
	var status atomic.Int32
	status.Swap(STARTING)


	disc, err := discordgo.New("Bot " + token)
	if err != nil {
		panic(err)
	}

	disc.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		status.Swap(READY)
		log.Printf("Session ready")
	}) 

	disc.AddHandler(interactionHandler) 

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

func interactionHandler(s *discordgo.Session, r *discordgo.InteractionCreate) {

	i := interaction{
		userId: "",
		username: "",
		content: "{none}",
		result: "{none}",
		timestamp: time.Now(),
	}

	defer func() { 
		respondCommand(i.result, s, r)
		go sendToWebhook(webhook, webhookName, i) 
	}()


	if r.User != nil {
		i.userId = r.User.ID
		i.username = r.User.Username
	} else if r.Member != nil && r.Member.User != nil && r.Member.User.ID == owner {
		i.userId = r.Member.User.ID
		i.username = r.Member.User.Username
	}

	if i.userId == "" {
		i.result = "Please add me as an application & DM me for a response!"
		return
	}

	if d := r.ApplicationCommandData(); d.Name == keyCheckCommand.Name {
		i.command = d.Name
		i.content = d.Options[0].StringValue()
		x := cooldownTracker[i.userId]
		if i.timestamp.Before(x) {
			i.result = fmt.Sprintf("You are on cooldown! You can try again <t:%d:R>", x.Unix())
			return
		}

		res := compareParts(key, item, i.content)
		if res == -1 {
			i.result = "Could not process key!"
		} else {
			i.result = fmt.Sprint(res)
			if i.userId != owner {
				cooldownTracker[i.userId] = i.timestamp.Add(time.Minute*time.Duration(cooldown))
			}
		}
	}
}

func respondCommand(message string, s *discordgo.Session, r *discordgo.InteractionCreate) {
	s.InteractionRespond(r.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
		},
	})

}

func sendToWebhook(webhook string, name string, i interaction) {
	content := fmt.Sprintf("New Interaction: ```%+v```", i)
	log.Print(content)
	if webhook == "" {
		return
	}

	type webhookBody struct {
		Username string `json:"username"`
		Content string `json:"content"`
	}

	body, err := json.Marshal(webhookBody{name, content})
	if err != nil {
		log.Print(err)
	}

	_, err = http.Post(webhook, "application/json", bytes.NewReader(body))
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
