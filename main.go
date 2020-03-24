package main

import (
	"flag"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path"
	"strings"

	"github.com/BurntSushi/toml"
)

func executeCommand(cmd string) ([]byte, error) {
	out, err := exec.Command("sh", "-c", cmd).CombinedOutput()
	if err != nil {
		return nil, err
	}
	return out, nil
}

func passwdeval(cmd string) string {
	if pass, err := executeCommand(cmd); err != nil {
		return ""
	} else {
		return strings.TrimSpace(string(pass))
	}
}

func main() {
	u, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}
	configPtr := flag.String("configFile", path.Join(u.HomeDir, ".config/go-imapnotify/config.toml"), "Path to the main configuration file")
	var conf AppConfig
	if _, err := toml.DecodeFile(*configPtr, &conf); err != nil {
		log.Fatal(err)
	}
	if conf.PasswdCmd != "" {
		conf.Password = passwdeval(conf.PasswdCmd)
	}

	app := newApp(conf)
	app.Start()
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	<-ch
	app.Stop()
}
