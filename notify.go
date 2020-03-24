package main

import (
	"log"
	"net"
	"strconv"
	"time"

	"github.com/emersion/go-imap"
	idle "github.com/emersion/go-imap-idle"
	"github.com/emersion/go-imap/client"
)

type AppConfig struct {
	Host         string
	Port         int
	Username     string
	PasswdCmd    string
	Password     string
	OnNotify     string
	OnNotifyPost string
	Boxes        []string // "*" to get update for all mailboxes
}

type App struct {
	connections []*client.Client
	conf        AppConfig

	stop chan struct{}
}

func getMailboxes(conf AppConfig) []string {
	var boxes []string
	c, err := client.DialTLS(net.JoinHostPort(conf.Host, strconv.Itoa(conf.Port)), nil)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		c.Timeout = 10 * time.Second
		c.Logout()
	}()

	if err := c.Login(conf.Username, conf.Password); err != nil {
		log.Fatal(err)
	}
	mailboxes := make(chan *imap.MailboxInfo, 10)
	done := make(chan error, 1)
	go func() {
		done <- c.List("", "*", mailboxes)
	}()

	for m := range mailboxes {
		boxes = append(boxes, m.Name)
	}

	if err := <-done; err != nil {
		log.Fatal(err)
	}
	return boxes
}

func parseMailBoxes(conf AppConfig) []string {
	for _, mailbox := range conf.Boxes {
		if mailbox == "*" {
			return getMailboxes(conf)
		}
	}
	return conf.Boxes
}

func newApp(conf AppConfig) *App {
	app := new(App)
	app.stop = make(chan struct{})
	app.conf = conf
	return app
}

func (app *App) Start() {
	boxes := parseMailBoxes(app.conf)
	if len(boxes) == 0 {
		log.Fatal("No mailbox")
	}
	for _, mailbox := range boxes {
		go app.newConnection(mailbox)
	}
}

func (app *App) newConnection(mailbox string) {
	conf := app.conf
	c, err := client.DialTLS(net.JoinHostPort(conf.Host, strconv.Itoa(conf.Port)), nil)
	if err != nil {
		log.Fatal(err)
	}
	if err := c.Login(conf.Username, conf.Password); err != nil {
		log.Fatal(err)
	}
	if _, err := c.Select(mailbox, false); err != nil {
		log.Fatal(err)
	}
	app.connections = append(app.connections, c)

	idleClient := idle.NewClient(c)
	// Create a channel to receive mailbox updates
	updates := make(chan client.Update)
	c.Updates = updates
	// Start idling
	done := make(chan error, 1)
	go func() {
		done <- idleClient.IdleWithFallback(nil, 0)
	}()

	// Listen for updates
waitLoop:
	for {
		select {
		case <-updates:
			// Here we use an external program to fetch mails.
			// More advanced use of the library can be seen at
			// https://github.com/emersion/go-imap-idle/issues/11#issuecomment-456090234
			go func() {
				if app.conf.OnNotify != "" {
					executeCommand(app.conf.OnNotify)
					if app.conf.OnNotifyPost != "" {
						executeCommand(app.conf.OnNotifyPost)
					}
				}
			}()
		case err := <-done:
			if err != nil {
				log.Fatal(err)
			}
			log.Println("Not idling anymore")
			return
		case <-app.stop:
			break waitLoop
		}
	}
}

func (app *App) Stop() {
	close(app.stop)
	for _, connection := range app.connections {
		// sometime it hangs indefinitely
		connection.Timeout = 10 * time.Second
		connection.Logout()
	}
}
