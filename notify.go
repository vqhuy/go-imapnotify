package main

import (
	"errors"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/emersion/go-imap"
	idle "github.com/emersion/go-imap-idle"
	"github.com/emersion/go-imap/client"
)

var errDisconnected = errors.New("imap: connection closed")

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

func NewApp(conf AppConfig) *App {
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
	app.start(boxes)
}

func (app *App) start(boxes []string) {
	for _, mailbox := range boxes {
		go func(mailbox string) {
			for {
				err := app.newConnection(mailbox)
				if err != errDisconnected {
					return
				}
			}
		}(mailbox)
	}
}

func (app *App) newConnection(mailbox string) error {
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
	index := app.addConnection(c)

	idleClient := idle.NewClient(c)
	// Create a channel to receive mailbox updates
	updates := make(chan client.Update)
	c.Updates = updates
	// Start idling
	stop := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- idleClient.IdleWithFallback(stop, 0)
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
			if err.Error() == errDisconnected.Error() {
				app.removeConnection(index)
				log.Println(err, "... reconnecting")
				return errDisconnected
			} else if err != nil {
				log.Fatal(err)
			}
			log.Println("Not idling anymore")
			return nil
		case <-app.stop:
			close(stop)
			break waitLoop
		}
	}
	return nil
}

func (app *App) addConnection(c *client.Client) int {
	app.connections = append(app.connections, c)
	return len(app.connections) - 1
}

func (app *App) removeConnection(i int) {
	app.connections[i] = app.connections[len(app.connections)-1]
	app.connections[len(app.connections)-1] = nil
	app.connections = app.connections[:len(app.connections)-1]
}

func (app *App) Stop() {
	close(app.stop)
	for _, connection := range app.connections {
		// sometime it hangs indefinitely
		connection.Timeout = 10 * time.Second
		connection.Logout()
	}
}
