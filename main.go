package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/snowflake/v2"
)

const SECONDS_IN_WEEK = 7 * 24 * 60 * 60

type Config struct {
	Account string
	Bsb     string
	UserId  string
	Port    string
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
	userId, ok := os.LookupEnv("DADDY_THANE_USER_ID")
	if !ok {
		panic("could not read user id from env")
	}
	conf.UserId = userId
	port, ok := os.LookupEnv("RENT_DISCORD_BOT_PORT")
	if !ok {
		slog.Error("could not read port, defaulting to 0")
		conf.Port = "0"
	} else {
		slog.Info("listening on port", slog.String("port", port))
		conf.Port = port
	}
}

type Renter struct {
	UserId       uint64 `json:"user_id"`
	ChannelId    uint64 `json:"channel_id"`
	Email        string `json:"email"`
	RentAmount   int64  `json:"weekly_rent_amt"`
	TimeLastPaid int64  `json:"unix_time_last_paid"`
}

func (r *Renter) calculateRent(curTime int64) float64 {
	rentInSeconds := float64(r.RentAmount) / float64(SECONDS_IN_WEEK)
	secondsSinceLastPay := curTime - r.TimeLastPaid
	amountToPay := rentInSeconds * float64(secondsSinceLastPay)
	return amountToPay
}

func postRent(renter *Renter, amount float64, client bot.Client) error {
	message := `
		# Rent Notice <@_USER>

        Amount owing: _OWING

        BSB: _BSB
        Account: _ACC

        Please contact <@_DADDY> within 24 hours if there are any issues! 😋

	`
	message = strings.Replace(message, "_USER", fmt.Sprintf("%d", renter.UserId), 1)
	message = strings.Replace(message, "_OWING", fmt.Sprintf("%.2f", amount), 1)
	message = strings.Replace(message, "_BSB", conf.Bsb, 1)
	message = strings.Replace(message, "_ACC", conf.Account, 1)
	message = strings.Replace(message, "_DADDY", conf.UserId, 1)

	var err error
	for range 3 {
		_, err = client.Rest().
			CreateMessage(snowflake.ID(renter.ChannelId), discord.NewMessageCreateBuilder().SetContent(message).Build())
		if err == nil {
			break
		}
		time.Sleep(time.Second)
	}

	if err != nil {
		slog.Error("error sending message", slog.Any("err", err))
	}
	return err
}

type Server struct {
	Client bot.Client
}

func unixToStr(t int64) string {
	ti := time.Unix(t, 0)
	f := ti.Format("2006-01-02 15:04:05")
	return f
}

func (s *Server) readSendRent(ctx context.Context, fp string) error {
	var renter Renter
	f, err := os.ReadFile(fp)
	if err != nil {
		return err
	}
	err = json.Unmarshal(f, &renter)
	if err != nil {
		return err
	}
	slog.InfoContext(
		ctx,
		"read from file ",
		slog.String("renter", renter.Email),
	)
	curTime := time.Now().Unix()
	amountToPay := renter.calculateRent(curTime)
	err = postRent(&renter, amountToPay, s.Client)
	if err != nil {
		return err
	}
	oldTime := renter.TimeLastPaid
	renter.TimeLastPaid = curTime
	renterBytes, err := json.Marshal(renter)
	if err != nil {
		return err
	}
	slog.InfoContext(
		ctx,
		"sent renter with amount",
		slog.String("renter", renter.Email),
		slog.Float64("amount", amountToPay),
		slog.String("time last paid", unixToStr(oldTime)),
		slog.String("time paid", unixToStr(renter.TimeLastPaid)),
	)
	err = os.WriteFile(fp, renterBytes, os.ModePerm)
	if err != nil {
		return err
	}
	return nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	bytedata, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "could not read file path", http.StatusBadRequest)
		return
	}
	err = s.readSendRent(r.Context(), string(bytedata))
	if err != nil {
		http.Error(w, "could not send rent", http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "sent rent to renter=%s", string(bytedata))
}

func setupHTTPServer(client bot.Client) {
	http.Handle("/", &Server{
		Client: client,
	})
	http.ListenAndServe(fmt.Sprintf(":%s", conf.Port), nil)
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
