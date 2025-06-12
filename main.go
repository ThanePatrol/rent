package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/snowflake/v2"
)

const SECONDS_IN_WEEK = 7 * 24 * 60 * 60

type Config struct {
	Account string
	Bsb     string
}

var conf Config

func init() {
	account, ok := os.LookupEnv("ACCOUNT")
	if !ok {
		panic("could not read account from env")
	}
	conf.Account = account
	bsb, ok := os.LookupEnv("BSB")
	if !ok {
		panic("could not read bsb from env")
	}
	conf.Bsb = bsb
}

type Renter struct {
	ChannelId     uint64 `json:"channel_id"`
	Email         string `json:"email"`
	RentInSeconds *float64
	RentAmount    int64 `json:"weekly_rent_amt"`
	TimeLastPaid  int64 `json:"unix_time_last_paid"`
}

func (r *Renter) calculateRent(curTime int64) float64 {
	if r.RentInSeconds == nil {
		a := float64(r.RentAmount) / float64(SECONDS_IN_WEEK)
		r.RentInSeconds = &a
	}
	secondsSinceLastPay := curTime - r.TimeLastPaid
	amountToPay := *r.RentInSeconds * float64(secondsSinceLastPay)
	return amountToPay
}

func postRent(renter *Renter, amount float64, client bot.Client) {
	message := `
        Amount owing: _OWING

        BSB: _BSB
        Account: _ACC

        Please contact me within 24 hours if there are any issues!

	`
	message = strings.Replace(message, "_OWING", fmt.Sprintf("%.2f", amount), 1)
	message = strings.Replace(message, "_BSB", conf.Bsb, 1)
	message = strings.Replace(message, "_ACC", conf.Account, 1)

	var err error
	for i := 0; i < 3; i++ {
		_, err = client.Rest().
			CreateMessage(snowflake.ID(renter.ChannelId), discord.NewMessageCreateBuilder().SetContent(message).Build())
		if err == nil {
			break
		}
		time.Sleep(time.Second)
	}

	if err != nil {
		slog.Error("error sending message", slog.Any("err", err))
		return
	}
}

type Server struct {
	Client bot.Client
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var renter Renter
	err := json.NewDecoder(r.Body).Decode(&renter)
	if err != nil {
		http.Error(w, "body was not a renter", http.StatusBadRequest)
		return
	}
	curTime := time.Now().Unix()
	amountToPay := renter.calculateRent(curTime)
	postRent(&renter, amountToPay, s.Client)
	fmt.Fprintf(w, "posted rent to channel_id=%d for renter=%s", renter.ChannelId, renter.Email)
}

func setupHTTPServer(client bot.Client) {
	http.Handle("/", &Server{
		Client: client,
	})
	http.ListenAndServe(":8080", nil)
}

func main() {
	slog.Info("starting example...")
	slog.Info("disgo version", slog.String("version", disgo.Version))

	client, err := disgo.New(os.Getenv("BOT_TOKEN"),
		bot.WithGatewayConfigOpts(
			gateway.WithIntents(
				gateway.IntentsAll,
				gateway.IntentGuildMessages,
				gateway.IntentMessageContent,
			),
		),
		bot.WithEventListenerFunc(onMessageCreate),
	)
	if err != nil {
		slog.Error("error while building disgo", slog.Any("err", err))
		return
	}

	defer client.Close(context.TODO())

	if err = client.OpenGateway(context.TODO()); err != nil {
		slog.Error("errors while connecting to gateway", slog.Any("err", err))
		return
	}

	go func() {
		setupHTTPServer(client)
	}()

	slog.Info("example is now running. Press CTRL-C to exit.")
	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-s
}

// TODO - Create private thread for each renter
// - ping renter on schedule (scheduling TBD)
// -

func onMessageCreate(event *events.MessageCreate) {
	if event.Message.Author.Bot {
		return
	}
	var message string
	if event.Message.Content == "ping" {
		message = "pong"
	} else if event.Message.Content == "pong" {
		message = "ping"
	}
	if message != "" {
		_, _ = event.Client().
			Rest().
			CreateMessage(event.ChannelID, discord.NewMessageCreateBuilder().SetContent(message).Build())
	}
}
